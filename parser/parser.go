package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/corpix/uarand"
	"github.com/pkg/errors"
)

type Trade struct {
	TradeQuantity   string  `json:"tradeQuantity"`
	SecurityID      string  `json:"securityID"`
	Price           float64 `json:"price"`
	TradeDate       string  `json:"tradeDate"`
	TimeOfExecution string  `json:"timeOfExecution"`
}

type Trades []*Trade

type FinraQP map[string]string
type FinraQS map[string][]FinraQP

const (
	FinraMarketsHost   = "finra-markets.morningstar.com"
	FinraBondSearchURL = "http://finra-markets.morningstar.com/bondSearch.jsp"
	FinraLoginURL      = "http://finra-markets.morningstar.com/finralogin.jsp"
)

const (
	FINRA_CFDUID = 1 << iota
	FINRA_QS_WSID
	FINRA_INSTID
	FINRA_CFRUID
	FINRA_SESSIONID
	FINRA_USRID
	FINRA_USRNAME
)

type Target struct {
	CUSIP     string
	StartDate string
	EndDate   string
}

type FinraClient struct {
	Jar    *cookiejar.Jar
	Client *http.Client
	*Target
	MaxLoginAttempts int
	loginAttempts    int
	ua               string
	recvLogin        chan int
	fetchTrades      chan *Target
	recvTrades       chan io.ReadCloser
	done             chan bool
}

func (fc *FinraClient) Start(cusip, startDate, endDate string) {
	fc.Target = &Target{
		CUSIP:     cusip,
		StartDate: startDate,
		EndDate:   endDate,
	}
	go fc.heartbeat()
	go fc.login()
	go fc.fetchListener()
}

func (fc *FinraClient) heartbeat() {
	for {
		select {
		case loginflag := <-fc.recvLogin:
			if loginflag == 0 {
				// successful login attempt
				fc.loginAttempts = 0
				fc.fetchTrades <- fc.Target
				break
			}

			// unsuccessful login attempt
			if fc.loginAttempts++; fc.loginAttempts > fc.MaxLoginAttempts {
				msg := fmt.Sprintf("Exceeded max login attempt threshold (%d)", fc.MaxLoginAttempts)
				log.Fatal(msg)
			}
			go fc.login()
		case body := <-fc.recvTrades:
			io.Copy(os.Stdout, body)
		}
	}
}

func (fc *FinraClient) login() {
	// build request
	req, err := http.NewRequest("GET", FinraLoginURL, nil)
	if err != nil {
		msg := fmt.Sprintf("Error getting FINRA login URL: %s", FinraLoginURL)
		log.Fatal(errors.Wrap(err, msg))
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

	// make request
	res, err := fc.Client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)

	// check for required login flags
	loginflag := FINRA_CFDUID | FINRA_QS_WSID | FINRA_INSTID | FINRA_CFRUID | FINRA_SESSIONID | FINRA_USRID | FINRA_USRNAME
	for _, c := range fc.Jar.Cookies(req.URL) {
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
	fc.recvLogin <- loginflag
}

func (fc *FinraClient) fetchListener() {
	for target := range fc.fetchTrades {
		// build payload
		secId := FinraQP{"Name": "securityId", "Value": target.CUSIP}
		td := FinraQP{"Name": "tradeDate", "minValue": target.StartDate, "maxValue": target.EndDate}
		qs := FinraQS{"Keywords": []FinraQP{secId, td}}

		b, err := json.Marshal(qs)
		if err != nil {
			log.Fatal(err)
		}
		qse := url.QueryEscape(string(b))

		v := url.Values{}
		v.Set("count", "20")
		v.Add("sortfield", "tradeDate")
		v.Add("sorttype", "2")
		v.Add("start", "0")
		v.Add("searchtype", "T")
		v.Add("query", qse)
		payload := strings.NewReader(v.Encode())

		// build request
		req, err := http.NewRequest("POST", FinraBondSearchURL, payload)
		if err != nil {
			log.Fatal(err)
		}

		// build referer string
		refv := url.Values{}
		refv.Set("ticker", target.CUSIP)
		refv.Set("startdate", target.StartDate)
		refv.Set("enddate", target.EndDate)
		refstr := "http://" + FinraMarketsHost + "/BondCenter/BondTradeActivitySearchResult.jsp?" + url.QueryEscape(refv.Encode())

		req.Header.Add("host", FinraMarketsHost)
		req.Header.Add("user-agent", fc.ua)
		req.Header.Add("accept", "text/plain, */*; q=0.01")
		req.Header.Add("accept-language", "en-US,en;q=0.5")
		req.Header.Add("accept-encoding", "gzip, deflate")
		req.Header.Add("content-type", "application/x-www-form-urlencoded")
		req.Header.Add("x-requested-with", "XMLHttpRequest")
		// req.Header.Add("referer", "http://finra-markets.morningstar.com/BondCenter/BondTradeActivitySearchResult.jsp?ticker=C765371&startdate=05%2F29%2F2018&enddate=05%2F29%2F2019")
		req.Header.Add("referer", refstr)
		req.Header.Add("cache-control", "no-cache,no-cache")
		req.Header.Add("connection", "keep-alive")

		res, err := fc.Client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()

		// handle response
		var reader io.ReadCloser
		switch res.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(res.Body)
			if err != nil {
				log.Fatal(err)
			}
			defer reader.Close()
		default:
			reader = res.Body
		}
		fc.recvTrades <- reader

	}
}

func NewFinraClient() *FinraClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{Jar: jar}
	return &FinraClient{
		Jar:              jar,
		Client:           client,
		MaxLoginAttempts: 5,
		ua:               uarand.GetRandom(),
		recvLogin:        make(chan int),
		recvTrades:       make(chan io.ReadCloser),
		fetchTrades:      make(chan *Target),
		done:             make(chan bool),
	}
}

func main() {
	// [TBU]
	// reverse loginflag bits
	// graceful exit event
	// graceful exit when exceed login attempts
	// target queue
	// check login status
	// stdin listener
	// wrap errors
	fc := NewFinraClient()
	fc.Start("C765371", "05/29/2018", "05/29/2019")
	<-fc.done
}
