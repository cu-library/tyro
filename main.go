// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

/*
This package implements a proxy for III's Sierra API.
It handles authentication and improves access to more commonly used data,
like item status.
*/
package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	l "github.com/cudevmaxwell/tyro/loglevel"
	"github.com/cudevmaxwell/tyro/sierraapi"
	"github.com/cudevmaxwell/tyro/tokenstore"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	//The prefix for all the curator environment variables
	EnvPrefix string = "TYRO_"

	//The default address to serve from
	DefaultAddress string = ":8877"

	//Will we allow raw mode access?
	DefaultRawAccess bool = false

	//The default Access-Control-Allow-Origin header (CORS)
	DefaultACAOHeader string = "*"
)

var (
	address      = flag.String("address", DefaultAddress, "Address for the server to bind on.")
	apiURL       = flag.String("url", sierraapi.DefaultURL, "API url.")
	certFile     = flag.String("certfile", "", "Certificate file location.")
	keyFile      = flag.String("keyfile", "", "Private key file location.")
	clientKey    = flag.String("key", "", "Client Key")
	clientSecret = flag.String("secret", "", "Client Secret")
	headerACAO   = flag.String("acaoheader", DefaultACAOHeader, "Access-Control-Allow-Origin Header for CORS. Multiple origins separated by ;")
	raw          = flag.Bool("raw", DefaultRawAccess, "Allow access to the raw Sierra API under /raw/")
	newLimit     = flag.Int("newlimit", 16, "The number of items to serve from the /new endpoint.")

	logFileLocation = flag.String("logfile", l.DefaultLogFileLocation, "Log file. By default, log messages will be printed to stdout.")
	logMaxSize      = flag.Int("logmaxsize", l.DefaultLogMaxSize, "The maximum size of log files before they are rotated, in megabytes.")
	logMaxBackups   = flag.Int("logmaxbackups", l.DefaultLogMaxBackups, "The maximum number of old log files to keep.")
	logMaxAge       = flag.Int("logmaxage", l.DefaultLogMaxAge, "The maximum number of days to retain old log files, in days.")
	logLevel        = flag.String("loglevel", "warn", "The maximum log level which will be logged. error < warn < info < debug < trace. For example, trace will log everything, info will log info, warn, and error.")

	tokenStore = tokenstore.NewTokenStore()
)

func init() {

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Tyro: A helper for Sierra APIs\nVersion 0.7.8\n\n")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "  The possible environment variables:")

		flag.VisitAll(func(f *flag.Flag) {
			uppercaseName := strings.ToUpper(f.Name)
			fmt.Fprintf(os.Stderr, "  %v%v\n", EnvPrefix, uppercaseName)
		})

		fmt.Fprintln(os.Stderr, "If a certificate file is provided, Tyro will attempt to use HTTPS.")
		fmt.Fprintln(os.Stderr, "The Access-Control-Allow-Origin header for CORS is only set for the /status/bib/[bibID], /status/item/[itemID] and /new endpoints.")
	}
}

func main() {

	flag.Parse()

	l.Set(l.ParseLogLevel(*logLevel))

	overrideUnsetFlagsFromEnvironmentVariables()

	l.SetupLumberjack(
		*logFileLocation,
		*logMaxSize,
		*logMaxBackups,
		*logMaxAge)

	l.Log("Starting Tyro", l.InfoMessage)
	l.Log("Serving on address: "+*address, l.InfoMessage)
	l.Log("Using Client Key: "+*clientKey, l.InfoMessage)
	l.Log("Using Client Secret: "+*clientSecret, l.InfoMessage)
	l.Log("Connecting to API URL: "+*apiURL, l.InfoMessage)
	l.Log("Using ACAO header: "+*headerACAO, l.InfoMessage)
	l.Log(fmt.Sprintf("Allowing access to raw Sierra API: %v", *raw), l.InfoMessage)

	if *clientKey == "" {
		log.Fatal("FATAL: A client key is required to authenticate against the Sierra API.")
	} else if *clientSecret == "" {
		log.Fatal("FATAL: A client secret is required to authenticate against the Sierra API.")
	}

	if *headerACAO == "*" {
		l.Log("Using \"*\" for \"Access-Control-Allow-Origin\" header. API will be public!", l.WarnMessage)
	}

	if *certFile != "" {
		l.Log("Going to try to serve through HTTPS", l.InfoMessage)
		l.Log("Using Certificate File: "+*certFile, l.InfoMessage)
		l.Log("Using Private Key File: "+*keyFile, l.InfoMessage)
	}

	parsedURL, err := parseURLandJoinToPath(*apiURL, sierraapi.TokenRequestEndpoint)
	if err != nil {
		log.Fatal("FATAL: Unable to parse API URL.")
	}

	tokenStore.Refresher(parsedURL.String(), *clientKey, *clientSecret)
	defer close(tokenStore.Refresh)

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/status/", statusHandler)
	http.HandleFunc("/status/item/", statusItemHandler)
	http.HandleFunc("/status/bib/", statusBibHandler)
	http.HandleFunc("/new", newBibsHandler)
	if *raw {
		l.Log("Allowing access to raw Sierra API.", l.WarnMessage)
		rawProxy := httputil.NewSingleHostReverseProxy(&url.URL{})
		rawProxy.Director = rawRewriter
		http.Handle("/raw/", rawProxy)
	}

	if *certFile == "" {
		log.Fatalf("FATAL: %v", http.ListenAndServe(*address, nil))
	} else {
		//Remove SSL 3.0 compatibility for POODLE exploit mitigation
		config := &tls.Config{MinVersion: tls.VersionTLS10}
		server := &http.Server{Addr: *address, Handler: nil, TLSConfig: config}
		log.Fatalf("FATAL: %v", server.ListenAndServeTLS(*certFile, *keyFile))
	}

}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "<html><head></head><body><pre>404 - Not Found</pre></body></html>")
		l.Log("404 Handler visited.", l.TraceMessage)
		return
	}
	l.Log("Home Handler visited.", l.TraceMessage)
	fmt.Fprint(w, "<html><head></head><body><h1>Welcome to Tyro! The Sierra API helper.</h1></body></html>")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	l.Log("Bare Status Handler visited.", l.TraceMessage)
	fmt.Fprint(w, "<html><head></head><body><pre>Available endpoints: /status/bib/[bibID] and /status/item/[itemID]</pre></body></html>")
}

func statusItemHandler(w http.ResponseWriter, r *http.Request) {

	setACAOHeader(w, r, *headerACAO)

	token, err := getTokenOrError(w, r)
	if err != nil {
		l.Log(err, l.ErrorMessage)
		return
	}

	itemID := strings.Split(r.URL.Path[len("/status/item/"):], "/")[0]
	if itemID == "" {
		http.Error(w, "Error, you need to provide an ItemID. /status/item/[ItemID]", http.StatusBadRequest)
		l.Log("Bad Request at /status/item/ handler, no ItemID provided.", l.TraceMessage)
		return
	}

	parsedAPIURL, err := parseURLandJoinToPath(*apiURL, sierraapi.ItemRequestEndpoint, itemID)
	if err != nil {
		http.Error(w, "Server Error.", http.StatusInternalServerError)
		l.Log("Internal Server Error at /status/item/ handler, unable to parse url.", l.DebugMessage)
		return
	}

	q := parsedAPIURL.Query()
	q.Set("suppressed", "false")
	q.Set("deleted", "false")
	parsedAPIURL.RawQuery = q.Encode()

	resp, err := sierraapi.SendRequestToAPI(parsedAPIURL.String(), token, w, r)
	if err != nil {
		l.Log(fmt.Sprintf("Internal Server Error at /status/item/, %v", err), l.ErrorMessage)
		return
	}
	if resp.StatusCode == http.StatusUnauthorized {
		http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
		tokenStore.Refresh <- struct{}{}
		l.Log("Token is out of date.", l.ErrorMessage)
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "No item records for that ItemID.", http.StatusNotFound)
		l.Log(fmt.Sprintf("No items records match ItemID %v", itemID), l.TraceMessage)
		return
	}

	var responseJSON sierraapi.ItemRecordIn

	err = json.NewDecoder(resp.Body).Decode(&responseJSON)
	defer resp.Body.Close()
	if err != nil {
		http.Error(w, "JSON Decoding Error", http.StatusInternalServerError)
		l.Log(fmt.Sprintf("Internal Server Error at /status/item/ handler, JSON Decoding Error: %v", err), l.WarnMessage)
		return
	}

	finalJSON, err := json.Marshal(responseJSON.Convert())
	if err != nil {
		http.Error(w, "JSON Encoding Error", http.StatusInternalServerError)
		l.Log(fmt.Sprintf("Internal Server Error at /status/item/ handler, JSON Encoding Error: %v", err), l.WarnMessage)
		return
	}

	l.Log(fmt.Sprintf("Sending response at /status/item handler: %v", responseJSON.Convert()), l.TraceMessage)

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Write(finalJSON)

}

func statusBibHandler(w http.ResponseWriter, r *http.Request) {

	setACAOHeader(w, r, *headerACAO)

	token, err := getTokenOrError(w, r)
	if err != nil {
		l.Log(err, l.ErrorMessage)
		return
	}

	bibID := strings.Split(r.URL.Path[len("/status/bib/"):], "/")[0]

	if bibID == "" {
		http.Error(w, "Error, you need to provide a BibID. /status/bib/[BidID]", http.StatusBadRequest)
		l.Log("Bad Request at /status/bib/ handler, no BidID provided.", l.TraceMessage)
		return
	}

	parsedAPIURL, err := parseURLandJoinToPath(*apiURL, sierraapi.ItemRequestEndpoint)
	if err != nil {
		http.Error(w, "Server Error.", http.StatusInternalServerError)
		l.Log("Internal Server Error at /status/bib/ handler, unable to parse url.", l.DebugMessage)
		return
	}

	q := parsedAPIURL.Query()
	q.Set("bibIds", bibID)
	q.Set("deleted", "false")
	q.Set("suppressed", "false")
	parsedAPIURL.RawQuery = q.Encode()

	resp, err := sierraapi.SendRequestToAPI(parsedAPIURL.String(), token, w, r)
	if err != nil {
		l.Log(fmt.Sprintf("Internal Server Error at /status/bib/, %v", err), l.ErrorMessage)
		return
	}
	if resp.StatusCode == http.StatusUnauthorized {
		http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
		tokenStore.Refresh <- struct{}{}
		l.Log("Token is out of date.", l.ErrorMessage)
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "No item records for that BibID.", http.StatusNotFound)
		l.Log(fmt.Sprintf("No items records match BibID %v", bibID), l.TraceMessage)
		return
	}

	var responseJSON sierraapi.ItemRecordsIn

	err = json.NewDecoder(resp.Body).Decode(&responseJSON)
	defer resp.Body.Close()
	if err != nil {
		http.Error(w, "JSON Decoding Error", http.StatusInternalServerError)
		l.Log(fmt.Sprintf("Internal Server Error at /status/bib/ handler, JSON Decoding Error: %v", err), l.WarnMessage)
		return
	}

	finalJSON, err := json.Marshal(responseJSON.Convert())
	if err != nil {
		http.Error(w, "JSON Encoding Error", http.StatusInternalServerError)
		l.Log(fmt.Sprintf("Internal Server Error at /status/bib/ handler, JSON Encoding Error: %v", err), l.WarnMessage)
		return
	}

	l.Log(fmt.Sprintf("Sending response at /status/bib/ handler: %v", responseJSON.Convert()), l.TraceMessage)

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Write(finalJSON)

}

func newBibsHandler(w http.ResponseWriter, r *http.Request) {

	setACAOHeader(w, r, *headerACAO)

	entries := make(map[int]sierraapi.BibRecordOut)

	entries, err := getNewItems(entries, time.Now(), w, r)
	if err != nil {
		return
	}

	var response sierraapi.BibRecordsOut

	for _, entry := range entries {
		response = append(response, entry)
	}

	sort.Sort(sort.Reverse(response))

	finalJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "JSON Encoding Error", http.StatusInternalServerError)
		l.Log(fmt.Sprintf("Internal Server Error at /new handler, JSON Encoding Error: %v", err), l.WarnMessage)
		return
	}

	l.Log(fmt.Sprintf("Sending response at /new handler: %v", response), l.TraceMessage)

	w.Header().Set("Content-Type", "application/json;charset=UTF-8 ")
	w.Write(finalJSON)
}

func getNumberOfEntries(date time.Time, w http.ResponseWriter, r *http.Request) (int, error) {

	type totalResponse struct {
		Total int `json:"total"`
	}

	token, err := getTokenOrError(w, r)
	if err != nil {
		l.Log(err, l.ErrorMessage)
		return 0, err
	}

	parsedAPIURL, err := parseURLandJoinToPath(*apiURL, sierraapi.BibRequestEndpoint)
	if err != nil {
		http.Error(w, "Server Error.", http.StatusInternalServerError)
		l.Log("Internal Server Error at /new handler, unable to parse url.", l.DebugMessage)
		return 0, err
	}

	q := parsedAPIURL.Query()
	q.Set("limit", "1")
	q.Set("offset", "0")
	q.Set("deleted", "false")
	q.Set("suppressed", "false")
	q.Set("createdDate", fmt.Sprintf("[%v,%v]", date.AddDate(0, 0, -1).Format(time.RFC3339), date.Format(time.RFC3339)))
	q.Set("fields", "default")
	parsedAPIURL.RawQuery = q.Encode()

	resp, err := sierraapi.SendRequestToAPI(parsedAPIURL.String(), token, w, r)
	if err != nil {
		l.Log(fmt.Sprintf("Internal Server Error at /new, %v", err), l.ErrorMessage)
		return 0, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
		tokenStore.Refresh <- struct{}{}
		l.Log("Token is out of date.", l.ErrorMessage)
		return 0, errors.New("Unauthorized")
	}

	var response totalResponse

	err = json.NewDecoder(resp.Body).Decode(&response)

	return response.Total, nil

}

func getNewItems(alreadyProcessed map[int]sierraapi.BibRecordOut, date time.Time, w http.ResponseWriter, r *http.Request) (map[int]sierraapi.BibRecordOut, error) {

	token, err := getTokenOrError(w, r)
	if err != nil {
		l.Log(err, l.ErrorMessage)
		return nil, err
	}

	parsedAPIURL, err := parseURLandJoinToPath(*apiURL, sierraapi.BibRequestEndpoint)
	if err != nil {
		http.Error(w, "Server Error.", http.StatusInternalServerError)
		l.Log("Internal Server Error at /new handler, unable to parse url.", l.DebugMessage)
		return nil, err
	}

	total, err := getNumberOfEntries(date, w, r)
	if err != nil {
		return nil, err
	}
	offset := 0
	needOneMoreDay := false

	if total >= *newLimit {
		offset = total - *newLimit
	} else {
		needOneMoreDay = true
	}

	q := parsedAPIURL.Query()
	q.Set("offset", strconv.Itoa(offset))
	q.Set("deleted", "false")
	q.Set("createdDate", fmt.Sprintf("[%v,%v]", date.AddDate(0, 0, -1).Format(time.RFC3339), date.Format(time.RFC3339)))
	q.Set("fields", "marc,default")
	q.Set("suppressed", "false")
	parsedAPIURL.RawQuery = q.Encode()

	resp, err := sierraapi.SendRequestToAPI(parsedAPIURL.String(), token, w, r)
	if err != nil {
		l.Log(fmt.Sprintf("Internal Server Error at /new, %v", err), l.ErrorMessage)
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
		tokenStore.Refresh <- struct{}{}
		l.Log("Token is out of date.", l.ErrorMessage)
		return nil, err
	}

	var response sierraapi.BibRecordsIn

	err = json.NewDecoder(resp.Body).Decode(&response)

	defer resp.Body.Close()
	if err != nil {
		http.Error(w, "JSON Decoding Error", http.StatusInternalServerError)
		l.Log(fmt.Sprintf("Internal Server Error at /new handler, JSON Decoding Error: %v", err), l.WarnMessage)
		return nil, err
	}

	entries := response.Convert()

	for _, entry := range *entries {
		alreadyProcessed[entry.BibID] = entry
	}

	if needOneMoreDay {
		return getNewItems(alreadyProcessed, date.Add(time.Duration(1435)*time.Minute*-1), w, r)
	} else {
		return alreadyProcessed, nil
	}

}

func rawRewriter(r *http.Request) {

	token, err := tokenStore.Get()
	if err != nil {
		l.Log("Error at /raw/ handler, token not yet generated.", l.DebugMessage)
	}

	parsedAPIURL, err := parseURLandJoinToPath(*apiURL, r.URL.Path[len("/raw/"):])
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}

	parsedAPIURL.RawQuery = r.URL.RawQuery

	r.URL = parsedAPIURL

	err = sierraapi.SetAuthorizationHeaders(r, r, token)
	if err != nil {
		l.Log("The remote address in an incoming request is not set properly", l.DebugMessage)
	}

	l.Log(fmt.Sprintf("Sending proxied request: %v", r), l.TraceMessage)

}

func overrideUnsetFlagsFromEnvironmentVariables() {
	listOfUnsetFlags := make(map[*flag.Flag]bool)

	//Ugly, but only way to get list of unset flags.
	flag.VisitAll(func(f *flag.Flag) { listOfUnsetFlags[f] = true })
	flag.Visit(func(f *flag.Flag) { delete(listOfUnsetFlags, f) })

	for k, _ := range listOfUnsetFlags {
		uppercaseName := strings.ToUpper(k.Name)
		environmentVariableName := fmt.Sprintf("%v%v", EnvPrefix, uppercaseName)
		environmentVariableValue := os.Getenv(environmentVariableName)
		if environmentVariableValue != "" {
			err := k.Value.Set(environmentVariableValue)
			if err != nil {
				log.Fatalf("FATAL: Unable to set configuration option %v from environment variable %v, which has a value of \"%v\"",
					k.Name, environmentVariableName, environmentVariableValue)
			}
		}
	}
}

func parseURLandJoinToPath(URL string, toJoin ...string) (*url.URL, error) {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return new(url.URL), errors.New("Unable to parse URL.")
	}
	for _, element := range toJoin {
		parsedURL.Path = path.Join(parsedURL.Path, element)
	}
	return parsedURL, nil
}

func getTokenOrError(w http.ResponseWriter, r *http.Request) (string, error) {

	token, err := tokenStore.Get()
	if err != nil {
		http.Error(w, "Token Error.", http.StatusInternalServerError)
		return token, err
	}
	if token == tokenstore.UninitialedTokenValue {
		l.Log("Waiting for token to initialize...", l.TraceMessage)
		select {
		case <-tokenStore.Initialized:
			tokenStore.Initialized <- struct{}{}
			token, err = tokenStore.Get()
			if err != nil {
				http.Error(w, "Token Error.", http.StatusInternalServerError)
				return token, err
			}
		case <-time.After(time.Second * 30):
			http.Error(w, "Token Error.", http.StatusInternalServerError)
			return token, errors.New("Unable to get token from TokenStore")
		}
	}

	return token, err
}

func setACAOHeader(w http.ResponseWriter, r *http.Request, headerConfig string) {
	if headerConfig == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if headerConfig != "" {
		possibleOrigins := strings.Split(headerConfig, ";")
		for _, okOrigin := range possibleOrigins {
			okOrigin = strings.TrimSpace(okOrigin)
			if (okOrigin != "") && (okOrigin == r.Header.Get("Origin")) {
				w.Header().Set("Access-Control-Allow-Origin", okOrigin)
			}
		}
	}
}
