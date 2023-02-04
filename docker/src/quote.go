package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/justinas/alice"
)

const (
	ALLOWED_METHOD = "GET"

	API_KEY string = "API_KEY"
	SYMBOL  string = "SYMBOL"
	NDAYS   string = "NDAYS"

	// This is the expected relative path of the certificate file allowing us to use TLS
	TLS_CERT_FILE string = "../certs/certbundle.pem"
	// This is the expected relative path of the TLS key file
	TLS_KEY_FILE string = "../certs/server.key"

	// The address we are listening on
	LISTEN_ADDR string = "LISTEN_ADDR"

	// These are the supported APIs
	HEALTHZ_API string = "/healthz"
	QUOTE_API   string = "/quote"

	// Base URL to retrieve stock quotes
	DEFAULT_BASE_URL string = "https://www.alphavantage.co/query/?function=TIME_SERIES_DAILY_ADJUSTED"

	// Time for quote server to respond
	// requestTimeout time.Duration = time.Second * 10
	// Retry interval.  TODO: Use an exponential backoff, and don't retry if unrecoverable error
	retryTimeout time.Duration = time.Second * 30

	// If listen address is not found in the environment.  Use default
	DEFAULT_LISTEN_ADDR string = ":8443"

	// Time date format
	YYYYMMDD = "2006-01-02"

	// Tag for daily closing price
	DAILY_CLOSING_PRICE_LABEL string = "4. close"
	// Insert average closing into the metadata (to make it look professional)
	METADATA_AVERAGE_CLOSING_PRICE_LABEL string = "6. Average Close"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
type EnvironmentVariables struct {
	// Required for accessing the quote server
	apiKey string
	// The symbol to fetch
	symbol string
	// Number of days to fetch
	nDays int

	// Address to listen on
	listenAddr string
}

// Options for the client connection
type HttpClientOptions struct {
	// Base URL of the quote server
	baseURL string
	// Insecure transport for testing
	insecureSkipVerify bool
	// connection timeout
	timeout time.Duration
}

// This is the format the server returns quotes in
type Quote struct {
	Metadata  map[string]interface{} `json:"Meta Data"`
	Dayquotes map[string]interface{} `json:"Time Series (Daily)"`
}

var (
	Env EnvironmentVariables
	Now func() time.Time

	ClientOptions HttpClientOptions
	Client        HTTPClient
)

// Setup the default
func init() {
	Now = time.Now

	ClientOptions = HttpClientOptions{
		baseURL:            DEFAULT_BASE_URL,
		insecureSkipVerify: false,
		timeout:            time.Second * 30,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ClientOptions.insecureSkipVerify},
	}

	Client = &http.Client{
		Timeout:   ClientOptions.timeout,
		Transport: tr,
	}
}

func getEnvironmentVariables() {
	nDays, err := strconv.Atoi(os.Getenv(NDAYS))
	if err != nil {
		log.Fatalf("unable to parse %s environment variable: %v", NDAYS, err)
	}
	if nDays < 1 {
		log.Fatalf("%s was not set or out of range, must be at least 1 day", NDAYS)
	}
	Env = EnvironmentVariables{
		apiKey: os.Getenv(API_KEY),
		symbol: os.Getenv(SYMBOL),
		nDays:  nDays,

		listenAddr: os.Getenv(LISTEN_ADDR),
	}
	if Env.apiKey == "" {
		log.Fatalf("%s was not set", API_KEY)
	}
	if Env.symbol == "" {
		log.Fatalf("%s was not set", SYMBOL)
	}

	if Env.listenAddr == "" {
		Env.listenAddr = DEFAULT_LISTEN_ADDR
	}
}

func getQuotesFromServer() (*Quote, int) {

	req, err := http.NewRequest("GET", ClientOptions.baseURL, nil)
	if err != nil {
		log.Printf("Got error %v", err.Error())
		return nil, http.StatusBadRequest
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("upgrade-insecure-requests", "1")

	// appending to existing query args
	q := req.URL.Query()
	q.Set("symbol", Env.symbol)
	q.Set("apikey", Env.apiKey)
	// Only return all available data if we really need it
	if Env.nDays > 100 {
		q.Set("outputsize", "full")
	}

	// assign encoded query string to http request
	req.URL.RawQuery = q.Encode()
	resp, err := Client.Do(req)
	if err != nil {
		log.Printf("quote server reponse error %v", err)
		return nil, http.StatusBadRequest
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// Something unexpected went wrong reading body?
		log.Printf("error reading quote server response body:%v", err)
		return nil, http.StatusInternalServerError
	}
	quote := new(Quote)

	if !json.Valid(body) {
		// handle the error here
		log.Println("invalid JSON returned from quote server")
		return nil, http.StatusInternalServerError
	}
	// TODO: Use a custom decoder to preserve key order
	err = json.Unmarshal(body, &quote)
	if err != nil {
		// Problems unmarshalling the resposne
		log.Printf("error unmarshalling quote server response: %v\n", err)
		return nil, http.StatusInternalServerError
	}
	return quote, resp.StatusCode
}

func healthCheckHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "ok\n")
}

func loggingHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		t1 := Now()
		next.ServeHTTP(w, r)
		t2 := Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	}
	return http.HandlerFunc(fn)
}

func recoverHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func requestQuote(w http.ResponseWriter, req *http.Request) {
	var longQuote *Quote
	// Retry until successful
	for {
		var code int
		longQuote, code = getQuotesFromServer()
		if code == http.StatusOK {
			break
		}
		log.Printf("remote service response:%d\n", code)
		time.Sleep(retryTimeout)
	}
	// Make sure we have enough data coming back
	daysAvailable := len(longQuote.Dayquotes)
	if daysAvailable < Env.nDays {
		errMsg := fmt.Sprintf("Requested %d days, only %d days of data is available", daysAvailable, Env.nDays)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	// Since maps are unordered, we must sort the keys
	days := make([]string, 0, daysAvailable)

	for day := range longQuote.Dayquotes {
		days = append(days, day)
	}
	sort.Strings(days)
	days = days[daysAvailable-Env.nDays:]

	// Trim longQuote to requested number of days
	shortQuote := Quote{}
	shortQuote.Metadata = longQuote.Metadata
	shortQuote.Dayquotes = make(map[string]interface{})
	var dayCloseSum float64
	for _, day := range days {
		dayQuote := longQuote.Dayquotes[day]
		shortQuote.Dayquotes[day] = dayQuote
		dayCloseString := dayQuote.(map[string]interface{})[DAILY_CLOSING_PRICE_LABEL].(string)
		dayClosePrice, err := strconv.ParseFloat(dayCloseString, 64)
		if err != nil {
			errMsg := fmt.Sprintf("unable to parse %s closing price %s: %v", day, dayCloseString, err)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		dayCloseSum += dayClosePrice
	}
	dayCloseAverage := strconv.FormatFloat(dayCloseSum/float64(Env.nDays), 'f', 2, 64)
	shortQuote.Metadata[METADATA_AVERAGE_CLOSING_PRICE_LABEL] = dayCloseAverage

	body, err := json.Marshal(shortQuote)
	if err != nil {
		errMsg := fmt.Sprintf("unable to marshal quote: %v", err)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	w.Write(body)

}

func setupHandlers(mux *http.ServeMux) {
	commonHandlers := alice.New(validator, loggingHandler, recoverHandler)
	mux.Handle(HEALTHZ_API, commonHandlers.ThenFunc(healthCheckHandler))
	mux.Handle(QUOTE_API, commonHandlers.ThenFunc(requestQuote))
}

func validator(next http.Handler) http.Handler {
	// For now, just check that we are using the GET method
	fn := func(w http.ResponseWriter, req *http.Request) {
		if (req.Method == "") || (req.Method == ALLOWED_METHOD) {
			// a valid request is passed on to next handler
			next.ServeHTTP(w, req)
		} else {
			// otherwise, respond with an error
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	}
	return http.HandlerFunc(fn)
}

func main() {
	// Read environment variables
	getEnvironmentVariables()
	mux := http.NewServeMux()
	setupHandlers(mux)

	log.Fatal(http.ListenAndServeTLS(Env.listenAddr, TLS_CERT_FILE, TLS_KEY_FILE, mux))
}
