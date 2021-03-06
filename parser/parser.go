package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/corpix/uarand"
	"github.com/pkg/errors"
)

// cli opts
var debug = flag.Bool("d", false, "debug mode")
var torConns = flag.Int("c", 1, "tor connections (default 1)")

type Trade struct {
	TradeQuantity   string  `json:"tradeQuantity"`
	SecurityID      string  `json:"securityID"`
	Price           float64 `json:"price"`
	TradeDate       string  `json:"tradeDate"`
	TimeOfExecution string  `json:"timeOfExecution"`
}

func (t *Trade) String() string {
	return fmt.Sprintf("%s [%s] %s %.3f %s", t.TradeDate, t.TimeOfExecution, t.SecurityID, t.Price, t.TradeQuantity)
}

type Trades struct {
	Columns []Trade `json:"Columns"`
	Rows    int     `json:"Rows"`
}

type TradeRes struct {
	T Trades `json:"T"`
}

type Target struct {
	CUSIP     string
	StartDate string
	EndDate   string
	nextReq   time.Time
	payload   *strings.Reader
	searchReq *http.Request
	fc        *FinraClient
}

func (tgt *Target) buildPayload() {
	secId := FinraQP{"Name": "securityId", "Value": tgt.CUSIP}
	td := FinraQP{"Name": "tradeDate", "minValue": tgt.StartDate, "maxValue": tgt.EndDate}
	qs := FinraQS{"Keywords": []FinraQP{secId, td}}

	b, err := json.Marshal(qs)
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("Error marshaling %+v", qs)))
	}
	qse := url.QueryEscape(string(b))

	v := url.Values{}
	v.Set("count", "20")
	v.Add("sortfield", "tradeDate")
	v.Add("sorttype", "2")
	v.Add("start", "0")
	v.Add("searchtype", "T")
	v.Add("query", qse)

	tgt.payload = strings.NewReader(v.Encode())
}

func (tgt *Target) buildSearchReq() {
	// build request
	req, err := http.NewRequest("POST", FinraBondSearchURL, tgt.payload)
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("Error building req %s", FinraBondSearchURL)))
	}

	// build referer string
	refv := url.Values{}
	refv.Set("ticker", tgt.CUSIP)
	refv.Set("startdate", tgt.StartDate)
	refv.Set("enddate", tgt.EndDate)
	refstr := "http://" + FinraMarketsHost + "/BondCenter/BondTradeActivitySearchResult.jsp?" + url.QueryEscape(refv.Encode())

	req.Header.Add("host", FinraMarketsHost)
	req.Header.Add("user-agent", tgt.fc.ua)
	req.Header.Add("accept", "text/plain, */*; q=0.01")
	req.Header.Add("accept-language", "en-US,en;q=0.5")
	req.Header.Add("accept-encoding", "gzip, deflate")
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("x-requested-with", "XMLHttpRequest")
	req.Header.Add("referer", refstr)
	req.Header.Add("cache-control", "no-cache,no-cache")
	req.Header.Add("connection", "keep-alive")

	tgt.searchReq = req
}

func (tgt *Target) handleSearchRes(res *http.Response) {
	defer res.Body.Close()

	var reader io.ReadCloser
	var err error
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(res.Body)
		if err != nil {
			log.Fatal(errors.Wrap(err, "Error reading res for trade fetch req"))
		}
		defer reader.Close()
	default:
		reader = res.Body
	}

	// read response body
	if b, err := ioutil.ReadAll(reader); err != nil {
		log.Fatal(errors.Wrap(err, "Error reading trade response body"))
	} else {
		tgt.parseTrades(b)
	}
}

func (tgt *Target) parseTrades(b []byte) {
	// fix malformed response
	b = bytes.Replace(b, []byte(`T:`), []byte(`"T":`), 1)
	var res TradeRes
	if err := json.Unmarshal(b, &res); err != nil {
		log.Fatal(errors.Wrap(err, "Error unmarshaling trades"))
	}

	// attempt to log in again if res is `{}`
	if bytes.Index(b, []byte(`"T":`)) == -1 {
		if *debug {
			fmt.Println("Received empty trade response.")
		}
	} else {
		for _, t := range res.T.Columns {
			tgt.fc.recvTrade <- t
		}
	}
}

const (
	TorDefaultPort       = 9050
	TorProxyStr          = "socks5://127.0.0.1:"
	TorCircuitSetupSleep = 5
	TorCircuitTimeout    = 5
)

const (
	FinraMarketsHost        = "finra-markets.morningstar.com"
	FinraBondSearchURL      = "http://finra-markets.morningstar.com/bondSearch.jsp"
	FinraLoginURL           = "http://finra-markets.morningstar.com/finralogin.jsp"
	FinraTorCircuitCheckURL = "https://check.torproject.org"
	FinraTimeoutDefault     = 10
)

const (
	FINRA_CFDUID = 1 << iota
	FINRA_QS_WSID
	FINRA_INSTID
	FINRA_CFRUID
	FINRA_SESSIONID
	FINRA_USRID
	FINRA_USRNAME
	FINRA_LOGGED_IN = FINRA_CFDUID | FINRA_QS_WSID | FINRA_INSTID | FINRA_CFRUID | FINRA_SESSIONID | FINRA_USRID | FINRA_USRNAME
)

type FinraQP map[string]string
type FinraQS map[string][]FinraQP

type FinraClient struct {
	Jar              *cookiejar.Jar
	Client           *http.Client
	Targets          []*Target
	MaxLoginAttempts int
	ua               string
	loginAttempts    int
	readyq           []Target
	recvLogin        chan bool
	recvTrade        chan Trade
	done             chan bool
	loginReq         *http.Request
}

func (fc *FinraClient) buildLoginReq() {
	// build request
	req, err := http.NewRequest("GET", FinraLoginURL, nil)
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("Error building login req: %s", FinraLoginURL)))
	}
	req.Header.Add("host", FinraMarketsHost)
	req.Header.Add("user-agent", fc.ua)
	req.Header.Add("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Add("accept-language", "en-US,en;q=0.5")
	req.Header.Add("accept-encoding", "gzip, deflate")
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("cache-control", "no-cache,no-cache")
	req.Header.Add("referer", FinraLoginURL)
	req.Header.Add("connection", "keep-alive")

	fc.loginReq = req
}

func (fc *FinraClient) login() {
	fc.fetch(fc.loginReq, fc.handleLoginRes)
}

func (fc *FinraClient) handleLoginRes(res *http.Response) {
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)

	// check for required login flags
	var loginflag int
	for _, c := range fc.Jar.Cookies(res.Request.URL) {
		switch c.Name {
		case "__cfduid":
			loginflag ^= FINRA_CFDUID
		case "qs_wsid":
			loginflag ^= FINRA_QS_WSID
		case "Instid":
			loginflag ^= FINRA_INSTID
		case "__cfruid":
			loginflag ^= FINRA_CFRUID
		case "SessionID":
			loginflag ^= FINRA_SESSIONID
		case "UsrID":
			loginflag ^= FINRA_USRID
		case "UsrName":
			loginflag ^= FINRA_USRNAME
		}
	}
	fc.loginAttempts++
	if loginflag == FINRA_LOGGED_IN {
		// successful login attempt
		if *debug {
			fmt.Printf("Login successful (attempt: %d)\n", fc.loginAttempts)
		}
		fc.loginAttempts = 0
		fc.recvLogin <- true
	} else if fc.loginAttempts > fc.MaxLoginAttempts {
		// unsuccessful login attempt
		log.Fatal(fmt.Sprintf("MaxLoginAttempts Exceeded (%d)", fc.MaxLoginAttempts))
	} else {
		// keep trying until exceed MaxLoginAttempts threshold
		if *debug {
			fmt.Printf("Login fail (attempt: %d; flag: %d)\n", fc.loginAttempts, loginflag)
		}
		fc.login()
	}
}

func (fc *FinraClient) AddTarget(cusip, startDate, endDate string) {
	tgt := fc.NewTarget(cusip, startDate, endDate)
	fc.Targets = append(fc.Targets, tgt)
}

func (fc *FinraClient) Start() {
	if err := fc.launchTor(); err != nil {
		log.Fatal(err)
	}

	if *debug {
		fmt.Println("Connected to tor")
	}

	go fc.tradeListener()
	go fc.checkTargets()
	go fc.login()
}

func (fc *FinraClient) tradeListener() {
	for trade := range fc.recvTrade {
		fmt.Println(&trade)
	}
}

func (fc *FinraClient) checkTargets() {
	if loggedIn := <-fc.recvLogin; !loggedIn {
		log.Fatal(errors.New("Unable to log in!"))
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var i int
	for {
		<-ticker.C
		target := fc.Targets[i%len(fc.Targets)]
		go fc.fetch(target.searchReq, target.handleSearchRes)
		i++
	}
}

func (fc *FinraClient) fetch(req *http.Request, rh func(*http.Response)) {
	res, err := fc.Client.Do(req)
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("Error with fetch request")))
	}
	rh(res)
}

func (fc *FinraClient) NewTarget(cusip, d0, d1 string) *Target {
	tgt := Target{
		CUSIP:     cusip,
		StartDate: d0,
		EndDate:   d1,
		fc:        fc,
		nextReq:   time.Now(),
	}
	tgt.buildPayload()
	tgt.buildSearchReq()
	return &tgt
}

func NewFinraClient(port int) *FinraClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	if err != nil {
		log.Fatal(errors.Wrap(err, "Err creating cookie jar"))
	}

	client := &http.Client{
		Transport: newTorTransport(port),
		Jar:       jar,
		Timeout:   time.Second * FinraTimeoutDefault,
	}
	var tgts []*Target
	var rtgts []Target
	fc := FinraClient{
		Jar:              jar,
		Client:           client,
		Targets:          tgts,
		MaxLoginAttempts: 5,
		ua:               uarand.GetRandom(),
		readyq:           rtgts,
		recvLogin:        make(chan bool),
		recvTrade:        make(chan Trade),
		done:             make(chan bool),
	}
	fc.buildLoginReq()
	return &fc
}

// launchTor starts tor and then polls circuit until either receiving a successful res without an error or
// TorCircuitTimeout has passed. If unsuccessful, a non-nil error is returned.
func (fc *FinraClient) launchTor() error {
	if *debug {
		fmt.Println("Checking for tor network")
	}

	// check for existing tor circuit
	if _, err := fc.Client.Get(FinraTorCircuitCheckURL); err == nil {
		if *debug {
			fmt.Println("Existing tor network found")
		}
		return err
	} else {
		log.Fatal(errors.Wrap(err, "Error trying to connect to tor circuit"))
	}

	// start tor launch
	cmd := exec.Command("tor")
	if err := cmd.Start(); err != nil {
		return err
	}

	// setup
	circuit := make(chan int)
	check := make(chan int)
	success := make(chan int)

	checker := func() {
		if _, err := fc.Client.Get(FinraTorCircuitCheckURL); err != nil {
			check <- 1
		} else {
			success <- 1
		}
	}

	// give tor time to setup circuit
	time.Sleep(time.Second * TorCircuitSetupSleep)
	timer := time.AfterFunc(time.Second*TorCircuitTimeout, func() {
		circuit <- 1
	})

	go checker()

	// keep checking circuit
	var err error
	var complete bool
	for {
		if complete {
			break
		}

		select {
		case <-circuit:
			err, complete = errors.New("TorCircuitTimeout expired"), true
		case <-check:
			fmt.Println("check called")
			go checker()
		case <-success:
			if *debug {
				fmt.Println("Tor network successfully launched")
			}
			// flush timer
			if !timer.Stop() {
				<-timer.C
			}
			err, complete = nil, true
		}
	}

	// cleanup, cleanup, everybody cleanup
	close(circuit)
	close(check)
	close(success)
	return err
}

func newTorTransport(port int) *http.Transport {
	torUrl := TorProxyStr + strconv.Itoa(port)
	url, err := url.Parse(torUrl)
	if *debug {
		fmt.Println("Building transport for tor conn:", url)
	}
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("Unable to parse TorProxyStr (%s)", torUrl)))
	}
	return &http.Transport{Proxy: http.ProxyURL(url)}
}

func main() {
	// [TBU]
	// tor cleanup script
	//
	// info logging
	// randomize reqs
	// input listener
	// add/remove targets
	flag.Parse()
	if *debug {
		fmt.Println("Parser launched in debug mode")
	}

	workers := make([]FinraClient, *torConns)
	targets := []string{"C765371", "C577245"}
	d0, d1 := "05/29/2018", "05/29/2019"
	for i := TorDefaultPort; i < TorDefaultPort+*torConns; i++ {
		fc := NewFinraClient(i)
		fc.AddTarget(targets[i-TorDefaultPort], d0, d1)
		fc.Start()
		workers[i-TorDefaultPort] = *fc
	}

	<-workers[0].done
}
