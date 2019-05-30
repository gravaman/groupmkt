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
	"strings"

	"github.com/corpix/uarand"
	"github.com/pkg/errors"
)

var debug = flag.Bool("d", false, "debug mode")

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
	FINRA_LOGGED_IN = FINRA_CFDUID | FINRA_QS_WSID | FINRA_INSTID | FINRA_CFRUID | FINRA_SESSIONID | FINRA_USRID | FINRA_USRNAME
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
	recvTrades       chan []byte
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
			fc.loginAttempts++
			if loginflag == FINRA_LOGGED_IN {
				// successful login attempt
				if *debug {
					fmt.Printf("Login successful (attempt: %d)\n", fc.loginAttempts)
				}
				fc.loginAttempts = 0
				fc.fetchTrades <- fc.Target
				break
			}

			// unsuccessful login attempt
			if fc.loginAttempts > fc.MaxLoginAttempts {
				log.Fatal(fmt.Sprintf("MaxLoginAttempts Exceeded (%d)", fc.MaxLoginAttempts))
			}

			// keep trying until exceed MaxLoginAttempts threshold
			if *debug {
				fmt.Printf("Login fail (attempt: %d; flag: %d)\n", fc.loginAttempts, loginflag)
			}
			go fc.login()
		case b := <-fc.recvTrades:
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
				go fc.login()
				break
			}

			// handle received trades
			fmt.Printf("Trades Received: %d\n", res.T.Rows)
			for _, t := range res.T.Columns {
				fmt.Println(&t)
			}
		}
	}
}

func (fc *FinraClient) login() {
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

	// make request
	res, err := fc.Client.Do(req)
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("Error making login req to: %s", FinraLoginURL)))
	}
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)

	// check for required login flags
	var loginflag int
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
		payload := strings.NewReader(v.Encode())

		// build request
		req, err := http.NewRequest("POST", FinraBondSearchURL, payload)
		if err != nil {
			log.Fatal(errors.Wrap(err, fmt.Sprintf("Error building req %s", FinraBondSearchURL)))
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
		req.Header.Add("referer", refstr)
		req.Header.Add("cache-control", "no-cache,no-cache")
		req.Header.Add("connection", "keep-alive")

		res, err := fc.Client.Do(req)
		if err != nil {
			log.Fatal(errors.Wrap(err, fmt.Sprintf("Error fetching from %s", FinraBondSearchURL)))
		}
		defer res.Body.Close()

		// handle response
		var reader io.ReadCloser
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
			fc.recvTrades <- b
		}
	}
}

func NewFinraClient() *FinraClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	if err != nil {
		log.Fatal(errors.Wrap(err, "Err creating cookie jar"))
	}

	client := &http.Client{Jar: jar}
	return &FinraClient{
		Jar:              jar,
		Client:           client,
		MaxLoginAttempts: 5,
		ua:               uarand.GetRandom(),
		recvLogin:        make(chan int),
		recvTrades:       make(chan []byte),
		fetchTrades:      make(chan *Target),
		done:             make(chan bool),
	}
}

func main() {
	// [TBU]
	// [1] px check loop
	// [2] target queue for a given client
	// [3] multiple clients for same host thru tor
	// info logging
	// reqs thru tor
	// randomize reqs
	// input listener
	flag.Parse()
	if *debug {
		fmt.Println("Parser launched in debug mode.")
	}

	fc := NewFinraClient()
	fc.Start("C765371", "05/29/2018", "05/29/2019")
	<-fc.done
}
