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
	"golang.org/x/net/html"
)

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

type FinraClient struct {
	Jar      *cookiejar.Jar
	Client   *http.Client
	LoggedIn bool
	ua       string
}

func (fc *FinraClient) Login() bool {
	// build request
	req, err := http.NewRequest("GET", FinraLoginURL, nil)
	if err != nil {
		log.Fatal(err)
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
	fc.LoggedIn = (loginflag == 0)
	return fc.LoggedIn
}

func (fc *FinraClient) FetchTrades(cusip, d0, d1 string) {
	// login
	a := 5
	for {
		if fc.LoggedIn {
			break
		}
		if a == 0 {
			log.Fatal("login attempts exceeded")
		}
		fc.Login()
		a--
	}

	// build payload
	secId := FinraQP{"Name": "securityId", "Value": cusip}
	td := FinraQP{"Name": "tradeDate", "minValue": d0, "maxValue": d1}
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

	req.Header.Add("host", FinraMarketsHost)
	req.Header.Add("user-agent", fc.ua)
	req.Header.Add("accept", "text/plain, */*; q=0.01")
	req.Header.Add("accept-language", "en-US,en;q=0.5")
	req.Header.Add("accept-encoding", "gzip, deflate")
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("x-requested-with", "XMLHttpRequest")
	req.Header.Add("referer", "http://finra-markets.morningstar.com/BondCenter/BondTradeActivitySearchResult.jsp?ticker=C765371&startdate=05%2F29%2F2018&enddate=05%2F29%2F2019")
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
	io.Copy(os.Stdout, reader)
}

func NewFinraClient() *FinraClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{Jar: jar}
	return &FinraClient{Jar: jar, Client: client, ua: uarand.GetRandom()}
}

func parseDoc(doc string) {
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		log.Fatal(err)
	}
	parseNode(node)
}

func parseNode(n *html.Node) {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" {
				fmt.Println(a.Val)
				break
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		parseNode(c)
	}
}

func main() {
	fc := NewFinraClient()
	fc.FetchTrades("C765371", "05/29/2018", "05/29/2019")
}
