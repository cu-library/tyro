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
	"github.com/cudevmaxwell/tyro/loglevel"
	"github.com/cudevmaxwell/tyro/tokenstore"
	"gopkg.in/cudevmaxwell-vendor/lumberjack.v2"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
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

	//API URL
	DefaultURL string = "https://sandbox.iii.com/iii/sierra-api/v1/"

	//API Endpoints
	TokenRequestEndpoint string = "token"
	ItemRequestEndpoint  string = "items"

	//Logging
	DefaultLogFileLocation string = "stdout"
	DefaultLogMaxSize      int    = 100
	DefaultLogMaxBackups   int    = 0
	DefaultLogMaxAge       int    = 0
)

var (
	address      = flag.String("address", DefaultAddress, "Address for the server to bind on.")
	apiURL       = flag.String("url", DefaultURL, "API url.")
	certFile     = flag.String("certfile", "", "Certificate file location.")
	keyFile      = flag.String("keyfile", "", "Private key file location.")
	clientKey    = flag.String("key", "", "Client Key")
	clientSecret = flag.String("secret", "", "Client Secret")
	headerACAO   = flag.String("acaoheader", DefaultACAOHeader, "Access-Control-Allow-Origin Header for CORS. Multiple origins separated by ;")
	raw          = flag.Bool("raw", DefaultRawAccess, "Allow access to the raw Sierra API under /raw/")

	logFileLocation = flag.String("logfile", DefaultLogFileLocation, "Log file. By default, log messages will be printed to stdout.")
	logMaxSize      = flag.Int("logmaxsize", DefaultLogMaxSize, "The maximum size of log files before they are rotated, in megabytes.")
	logMaxBackups   = flag.Int("logmaxbackups", DefaultLogMaxBackups, "The maximum number of old log files to keep.")
	logMaxAge       = flag.Int("logmaxage", DefaultLogMaxAge, "The maximum number of days to retain old log files, in days.")
	logLevel        = flag.String("loglevel", "warn", "The maximum log level which will be logged. error < warn < info < debug < trace. For example, trace will log everything, info will log info, warn, and error.")

	tokenStore = tokenstore.NewTokenStore()

	LogMessageLevel loglevel.LogLevel
)

func init() {

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Tyro: A helper for Sierra APIs\n\n")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "  The possible environment variables:")

		flag.VisitAll(func(f *flag.Flag) {
			uppercaseName := strings.ToUpper(f.Name)
			fmt.Fprintf(os.Stderr, "  %v%v\n", EnvPrefix, uppercaseName)
		})

		fmt.Fprintln(os.Stderr, "If a certificate file is provided, Tyro will attempt to use HTTPS.")
		fmt.Fprintln(os.Stderr, "The Access-Control-Allow-Origin header for CORS is only set for the /status/[bibID] endpoint.")
	}
}

func main() {

	flag.Parse()

	LogMessageLevel = loglevel.ParseLogLevel(*logLevel)

	overrideUnsetFlagsFromEnvironmentVariables()

	if *logFileLocation != DefaultLogFileLocation {
		log.SetOutput(&lumberjack.Logger{
			Filename:   *logFileLocation,
			MaxSize:    *logMaxSize,
			MaxBackups: *logMaxBackups,
			MaxAge:     *logMaxAge,
		})
	} else {
		log.SetOutput(os.Stdout)
	}

	logM("Starting Tyro", loglevel.InfoMessage)
	logM("Serving on address: "+*address, loglevel.InfoMessage)
	logM("Using Client Key: "+*clientKey, loglevel.InfoMessage)
	logM("Using Client Secret: "+*clientSecret, loglevel.InfoMessage)
	logM("Connecting to API URL: "+*apiURL, loglevel.InfoMessage)
	logM("Using ACAO header: "+*headerACAO, loglevel.InfoMessage)
	logM(fmt.Sprintf("Allowing access to raw Sierra API: %v", *raw), loglevel.InfoMessage)

	if *clientKey == "" {
		log.Fatal("FATAL: A client key is required to authenticate against the Sierra API.")
	} else if *clientSecret == "" {
		log.Fatal("FATAL: A client secret is required to authenticate against the Sierra API.")
	}

	if *headerACAO == "*" {
		logM("Using \"*\" for \"Access-Control-Allow-Origin\" header. API will be public!", loglevel.WarnMessage)
	}

	if *certFile != "" {
		logM("Going to try to serve through HTTPS", loglevel.InfoMessage)
		logM("Using Certificate File: "+*certFile, loglevel.InfoMessage)
		logM("Using Private Key File: "+*keyFile, loglevel.InfoMessage)
	}

	parsedURL, err := parseURLandEndpoint(*apiURL, TokenRequestEndpoint)
	if err != nil {
		log.Fatal("FATAL: Unable to parse API URL.")
	}

	go func() {
		for m := range tokenStore.LogMessages {
			logM(m.Message, m.Level)
		}
	}()
	tokenStore.Refresher(parsedURL.String(), *clientKey, *clientSecret)

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/status/", statusHandler)
	if *raw {
		logM("Allowing access to raw Sierra API.", loglevel.WarnMessage)
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
		return
	}

	fmt.Fprint(w, "<html><head></head><body><h1>Welcome to Tyro! The Sierra API helper.</h1></body></html>")

}

func statusHandler(w http.ResponseWriter, r *http.Request) {

	token, err := tokenStore.Get()
	if err != nil {
		http.Error(w, "Token Error.", http.StatusInternalServerError)
		logM("Internal Server Error at /status/ handler, token error.", loglevel.DebugMessage)
		return
	}
	if token == tokenstore.UninitialedTokenValue {
		logM("Waiting for token to initialize...", loglevel.TraceMessage)
		select {
		case <-tokenStore.Initialized:
			tokenStore.Initialized <- struct{}{}
			token, err = tokenStore.Get()
			if err != nil {
				http.Error(w, "Token Error.", http.StatusInternalServerError)
				logM("Internal Server Error at /status/ handler, token error.", loglevel.DebugMessage)
				return
			}
		case <-time.After(time.Second * 30):
			http.Error(w, "Token Error.", http.StatusInternalServerError)
			logM("Internal Server Error at /status/ handler, token error.", loglevel.DebugMessage)
			return
		}
	}

	bibID := strings.Split(r.URL.Path[len("/status/"):], "/")[0]

	if bibID == "" {
		http.Error(w, "Error, you need to provide a Bib ID. /status/[BidID]", http.StatusBadRequest)
		logM("Bad Request at /status/ handler, no BidID provided.", loglevel.TraceMessage)
		return
	}

	parsedAPIURL, err := parseURLandEndpoint(*apiURL, ItemRequestEndpoint)
	if err != nil {
		http.Error(w, "Server Error.", http.StatusInternalServerError)
		logM("Internal Server Error at /status/ handler, unable to parse url.", loglevel.DebugMessage)
		return
	}

	q := parsedAPIURL.Query()
	q.Set("bibIds", bibID)
	q.Set("deleted", "false")
	parsedAPIURL.RawQuery = q.Encode()

	getItemStatus, err := http.NewRequest("GET", parsedAPIURL.String(), nil)
	if err != nil {
		http.Error(w, "Request failed.", http.StatusInternalServerError)
		logM("Internal Server Error at /status/ handler, request failed.", loglevel.DebugMessage)
		return
	}

	setAuthorizationHeaders(getItemStatus, r, token)

	client := &http.Client{}
	resp, err := client.Do(getItemStatus)
	if err != nil {
		http.Error(w, "Error querying Sierra API", http.StatusInternalServerError)
		logM(fmt.Sprintf("Internal Server Error at /status/ handler, GET against itemStatusURL failed: %v", err), loglevel.WarnMessage)
		return
	}

	if resp.StatusCode == http.StatusUnauthorized {
		http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
		logM("Internal Server Error at /status/ handler, token is out of date:", loglevel.WarnMessage)
		tokenStore.Refresh <- struct{}{}
		return
	}

	var responseJSON struct {
		Entries []struct {
			CallNumber string `json:"callNumber"`
			Status     struct {
				DueDate time.Time `json:"duedate"`
			} `json:"status"`
			Location struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"entries"`
	}

	err = json.NewDecoder(resp.Body).Decode(&responseJSON)
	defer resp.Body.Close()
	if err != nil {
		http.Error(w, "JSON Decoding Error", http.StatusInternalServerError)
		logM(fmt.Sprintf("Internal Server Error at /status/ handler, JSON Decoding Error: %v", err), loglevel.WarnMessage)
		return
	}

	type Entry struct {
		CallNumber string
		Status     string
		Location   string
	}

	var statusJSON struct {
		Entries []Entry
	}

	for _, responseEntry := range responseJSON.Entries {
		newEntry := Entry{}
		newEntry.CallNumber = responseEntry.CallNumber
		newEntry.CallNumber = strings.Replace(newEntry.CallNumber, "|a", " ", -1)
		newEntry.CallNumber = strings.Replace(newEntry.CallNumber, "|b", " ", -1)
		if responseEntry.Status.DueDate.IsZero() {
			newEntry.Status = "IN LIBRARY"
		} else {
			newEntry.Status = "DUE " + responseEntry.Status.DueDate.Format("January 2, 2006")
		}
		newEntry.Location = responseEntry.Location.Name

		statusJSON.Entries = append(statusJSON.Entries, newEntry)
	}

	finaljson, err := json.Marshal(statusJSON)
	if err != nil {
		http.Error(w, "JSON Encoding Error", http.StatusInternalServerError)
		logM(fmt.Sprintf("Internal Server Error at /status/ handler, JSON Encoding Error: %v", err), loglevel.WarnMessage)
		return
	}

	if *headerACAO == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if *headerACAO != "" {
		possibleOrigins := strings.Split(*headerACAO, ";")
		for _, okOrigin := range possibleOrigins {
			okOrigin = strings.TrimSpace(okOrigin)
			if (okOrigin != "") && (okOrigin == r.Header.Get("Origin")) {
				w.Header().Set("Access-Control-Allow-Origin", okOrigin)
			}
		}
	}

	logM(fmt.Sprintf("Sending response at /status/ handler: %v", statusJSON), loglevel.TraceMessage)

	w.Header().Set("Content-Type", "application/json")
	w.Write(finaljson)

}

func rawRewriter(r *http.Request) {

	token, err := tokenStore.Get()
	if err != nil {
		logM("Error at /raw/ handler, token not yet generated.", loglevel.DebugMessage)
	}

	parsedAPIURL, err := parseURLandEndpoint(*apiURL, r.URL.Path[len("/raw/"):])
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}

	parsedAPIURL.RawQuery = r.URL.RawQuery

	r.URL = parsedAPIURL

	setAuthorizationHeaders(r, r, token)

	logM(fmt.Sprintf("Sending proxied request: %v", r), loglevel.TraceMessage)

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

//Log a message if the level is below or equal to the set LogMessageLevel
func logM(message interface{}, messagelevel loglevel.LogLevel) {
	if messagelevel <= LogMessageLevel {
		log.Printf("%v: %v\n", strings.ToUpper(messagelevel.String()), message)
	}
}

func parseURLandEndpoint(URL, endpoint string) (*url.URL, error) {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return new(url.URL), errors.New("Unable to parse URL.")
	}
	parsedURL.Path = path.Join(parsedURL.Path, endpoint)
	return parsedURL, nil
}

//Set the required Authorization headers.
//This includes the Bearer token, User-Agent, and X-Forwarded-For
//I'd rather this be a function on http.Request types, but Go
//forbids that without embedding in a new type.
func setAuthorizationHeaders(nr, or *http.Request, t string) {
	nr.Header.Add("Authorization", "Bearer "+t)
	nr.Header.Add("User-Agent", "Tyro")

	originalForwardFor := or.Header.Get("X-Forwarded-For")
	if originalForwardFor == "" {
		ip, _, err := net.SplitHostPort(or.RemoteAddr)
		if err != nil {
			logM("The remote address in an incoming request is not set properly", loglevel.TraceMessage)
		} else {
			nr.Header.Add("X-Forwarded-For", ip)
		}
	} else {
		nr.Header.Add("X-Forwarded-For", originalForwardFor)
	}
}
