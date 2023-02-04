package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	TEST_API_KEY     = "TestApiKey"
	TEST_SYMBOL      = "TestSYM"
	TEST_NDAYS       = 5
	TEST_LISTEN_ADDR = "127.0.0.1:8443"

	METADATA = `{
		"Meta Data": 
		{ 
			"1. Information": "Daily Time Series with Splits and Dividend Events",
			"2. Symbol": "MSFT",
			"3. Last Refreshed": "2023-02-02",
			"4. Output Size": "Compact",
			"5. Time Zone": "US/Eastern"
		},
		"Time Series (Daily)":
		{
			"2023-01-24": 
			{
				"1. open": "242.5",
				"2. high": "243.95",
				"3. low": "240.44",
				"4. close": "242.04",
				"5. adjusted close": "242.04",
				"6. volume": "40234444",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			},
			"2023-01-25": {
				"1. open": "234.48",
				"2. high": "243.3",
				"3. low": "230.9",
				"4. close": "240.61",
				"5. adjusted close": "240.61",
				"6. volume": "66526641",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			},
			"2023-01-26": {
				"1. open": "243.65",
				"2. high": "248.31",
				"3. low": "242.0",
				"4. close": "248.0",
				"5. adjusted close": "248.0",
				"6. volume": "33454491",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			},
			"2023-01-27":
			{
				"1. open": "258.82",
				"2. high": "264.69",
				"3. low": "257.25",
				"4. close": "264.6",
				"5. adjusted close": "264.6",
				"6. volume": "3913618",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			},
			"2023-01-30": {
				"1. open": "244.51",
				"2. high": "245.6",
				"3. low": "242.2",
				"4. close": "242.71",
				"5. adjusted close": "242.71",
				"6. volume": "25867365",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			  },
			  "2023-01-31": {
				"1. open": "243.45",
				"2. high": "247.95",
				"3. low": "242.945",
				"4. close": "247.81",
				"5. adjusted close": "247.81",
				"6. volume": "26541072",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			  },
			  "2023-02-01": {
				"1. open": "248.0",
				"2. high": "255.18",
				"3. low": "245.47",
				"4. close": "252.75",
				"5. adjusted close": "252.75",
				"6. volume": "31259912",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			  },
			  "2023-02-02": {
				"1. open": "258.82",
				"2. high": "264.69",
				"3. low": "257.25",
				"4. close": "264.6",
				"5. adjusted close": "264.6",
				"6. volume": "39136189",
				"7. dividend amount": "0.0000",
				"8. split coefficient": "1.0"
			  }
		}
	}`

	EXPECTED_DAYS_AVAIL             int = 8
	EXPECTED_HEALTH_CHECK_RESPONSE      = "ok\n"
	DUMMY_URL                           = "https://localhost:8443/dummy"
	EXPECTED_LOG_RESPONSE               = "[GET] \"" + DUMMY_URL + "\" 0s"
	TEST_PANIC_MESSAGE                  = "Test panic"
	EXPECTED_RECOVER_PANIC_RESPONSE     = "panic: " + TEST_PANIC_MESSAGE
	EXPECTED_LOG_HEALTHZ_RESPONSE       = "[GET] \"" + HEALTHZ_API + "\" 0s"
)

func mockLogger(t *testing.T) (*bufio.Scanner, *os.File, *os.File) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Error("couldn't get os Pipe:", err)
	}
	log.SetOutput(writer)

	return bufio.NewScanner(reader), reader, writer
}

func resetLogger(reader *os.File, writer *os.File) {
	err := reader.Close()
	if err != nil {
		fmt.Println("error closing reader:", err)
	}
	if err = writer.Close(); err != nil {
		fmt.Println("error closing writer:", err)
	}
	log.SetOutput(os.Stderr)
}

func TestGetEnvironmentVariables(t *testing.T) {
	t.Setenv(API_KEY, TEST_API_KEY)
	t.Setenv(SYMBOL, TEST_SYMBOL)
	t.Setenv(NDAYS, strconv.Itoa(TEST_NDAYS))

	getEnvironmentVariables()

	if Env.apiKey != TEST_API_KEY {
		t.Error("expected:", TEST_API_KEY, "got", Env.apiKey)
	}

	if Env.symbol != TEST_SYMBOL {
		t.Error("expected:", TEST_SYMBOL, "got", Env.symbol)
	}

	if Env.nDays != TEST_NDAYS {
		t.Error("expected:", TEST_NDAYS, "got", Env.nDays)
	}
	if Env.listenAddr != DEFAULT_LISTEN_ADDR {
		t.Error("expected:", DEFAULT_LISTEN_ADDR, "got", Env.listenAddr)
	}
	// Set non-default listen address
	t.Setenv(LISTEN_ADDR, TEST_LISTEN_ADDR)

	getEnvironmentVariables()

	if Env.listenAddr != TEST_LISTEN_ADDR {
		t.Error("expected:", TEST_LISTEN_ADDR, "got", Env.listenAddr)
	}
}

// TestGetQuotesFromServer tests the client code
func TestGetQuotesFromServer(t *testing.T) {

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, METADATA)
	}))
	defer ts.Close()

	ClientOptions = HttpClientOptions{
		baseURL:            ts.URL,
		insecureSkipVerify: true,
		timeout:            60 * time.Second,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ClientOptions.insecureSkipVerify},
	}

	Client = &http.Client{
		Timeout:   ClientOptions.timeout,
		Transport: tr,
	}

	quote, code := getQuotesFromServer()
	if code != http.StatusOK {
		t.Error("expected:", http.StatusOK, "got", code)
	}
	daysAvailable := len(quote.Dayquotes)
	if daysAvailable != EXPECTED_DAYS_AVAIL {
		t.Error("expected", EXPECTED_DAYS_AVAIL, "days, got", daysAvailable)
	}
}

func TestHealthCheckHandler(t *testing.T) {

	expected := []byte(EXPECTED_HEALTH_CHECK_RESPONSE)
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	res := httptest.NewRecorder()
	healthCheckHandler(res, req)
	if res.Code != http.StatusOK {
		t.Errorf("response code was %v; want 200", res.Code)
	}
	if !bytes.Equal(expected, res.Body.Bytes()) {
		t.Errorf("response body was '%v'; want '%v'", res.Body, expected)
	}
}

func TestLoggingHandler(t *testing.T) {
	scanner, reader, writer := mockLogger(t)
	defer resetLogger(reader, writer)
	Now = func() time.Time {
		return time.Date(2023, 2, 2, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		Now = time.Now
	}()
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	testHandler := http.HandlerFunc(mockHandler)
	retHandler := loggingHandler(testHandler)
	r, err := http.NewRequest("GET", DUMMY_URL, nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	retHandler.ServeHTTP(w, r)
	scanner.Scan()
	msg := scanner.Text()
	if !strings.Contains(msg, EXPECTED_LOG_RESPONSE) {
		t.Error("response:", msg, "should contain:", EXPECTED_LOG_RESPONSE)
	}
}

func TestRecoverHandler(t *testing.T) {
	scanner, reader, writer := mockLogger(t)
	defer resetLogger(reader, writer)
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a panic that we should recover from
		panic(TEST_PANIC_MESSAGE)
	})
	testHandler := http.HandlerFunc(mockHandler)
	retHandler := recoverHandler(testHandler)
	r, err := http.NewRequest("GET", DUMMY_URL, nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	retHandler.ServeHTTP(w, r)
	scanner.Scan()
	msg := scanner.Text()
	if !strings.Contains(msg, EXPECTED_RECOVER_PANIC_RESPONSE) {
		t.Error("response:", msg, "should contain:", EXPECTED_RECOVER_PANIC_RESPONSE)
	}
}

func TestRequestQuote(t *testing.T) {
	Env.nDays = 5
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, METADATA)
	}))
	defer ts.Close()

	ClientOptions = HttpClientOptions{
		baseURL:            ts.URL,
		insecureSkipVerify: true,
		timeout:            60 * time.Second,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ClientOptions.insecureSkipVerify},
	}

	Client = &http.Client{
		Timeout:   ClientOptions.timeout,
		Transport: tr,
	}

	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Error(err)
	}
	res := httptest.NewRecorder()
	requestQuote(res, req)
	if res.Code != http.StatusOK {
		t.Errorf("response code was %v; want 200", res.Code)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		// Something unexpected went wrong reading body?
		t.Error("error reading quote server response body:", err)
	}
	if !json.Valid(body) {
		// handle the error here
		t.Error("invalid JSON returned from quote server")
	}
	// TODO: Add more specific checks to validate the returned JSON
}

func TestSetupHandlers(t *testing.T) {
	// Make sure that both the logger and healthz handlers are called!
	scanner, reader, writer := mockLogger(t)
	defer resetLogger(reader, writer)
	Now = func() time.Time {
		return time.Date(2023, 2, 2, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		Now = time.Now
	}()
	mux := http.NewServeMux()
	setupHandlers(mux)

	r, err := http.NewRequest("GET", HEALTHZ_API, nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("response code was %v; want 200", w.Code)
	}
	expected := []byte(EXPECTED_HEALTH_CHECK_RESPONSE)
	if !bytes.Equal(expected, w.Body.Bytes()) {
		t.Errorf("response body was '%v'; want '%v'", w.Body, expected)
	}
	scanner.Scan()
	msg := scanner.Text()
	if !strings.Contains(msg, EXPECTED_LOG_HEALTHZ_RESPONSE) {
		t.Error("response:", msg, "should contain:", EXPECTED_LOG_HEALTHZ_RESPONSE)
	}
}

func TestValidator(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok\n")
	})
	testHandler := http.HandlerFunc(mockHandler)
	retHandler := validator(testHandler)
	r, err := http.NewRequest("GET", DUMMY_URL, nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	retHandler.ServeHTTP(w, r)
	expected := []byte(EXPECTED_HEALTH_CHECK_RESPONSE)
	if !bytes.Equal(expected, w.Body.Bytes()) {
		t.Errorf("response body was '%v'; want '%v'", w.Body, expected)
	}
	r, err = http.NewRequest("PUT", DUMMY_URL, nil)
	if err != nil {
		t.Error(err)
	}
	w = httptest.NewRecorder()
	retHandler.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Error("response code was ", w.Code, " want ", http.StatusMethodNotAllowed)
	}
	r, err = http.NewRequest("POST", DUMMY_URL, nil)
	if err != nil {
		t.Error(err)
	}
	w = httptest.NewRecorder()
	retHandler.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Error("response code was ", w.Code, " want ", http.StatusMethodNotAllowed)
	}
}
