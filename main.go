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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

//The prefix for all the curator environment variables
const EnvPrefix string = "TYRO_"

//The default address to serve from
const DefaultAddress string = ":8877"

//Will we run in verbose mode?
const DefaultVerbose bool = false

//The default Access-Control-Allow-Origin header (CORS)
const DefaultACAOHeader string = "*"

//API URL
const DefaultURL string = "https://sandbox.iii.com/iii/sierra-api/v1/"

//API Endpoints
const TokenRequestEndpoint string = "token"
const ItemRequestEndpoint string = "items"

var (
	address      = flag.String("address", DefaultAddress, "Address for the server to bind on.")
	verbose      = flag.Bool("v", DefaultVerbose, "Print debugging information.")
	apiURL       = flag.String("url", DefaultURL, "API url.")
	certFile     = flag.String("certfile", "", "Certificate file location.")
	keyFile      = flag.String("keyfile", "", "Private key file location.")
	clientKey    = flag.String("key", "", "Client Key")
	clientSecret = flag.String("secret", "", "Client Secret")
	headerACAO   = flag.String("acaoheader", DefaultACAOHeader, "Access-Control-Allow-Origin Header for CORS. Multiple origins separated by ;")

	templates = template.Must(template.ParseGlob("templates/*.html"))

	tokenChan        chan string
	refreshTokenChan chan bool
)

func main() {

	flag.Usage = func() {
		fmt.Print("Tyro: A helper for Sierra APIs\n\n")
		flag.PrintDefaults()
		fmt.Println("The possible environment variables:")
		fmt.Println("TYRO_ADDRESS, TYRO_VERBOSE, TYRO_KEY, TYRO_SECRET, TYRO_URL, TYRO_CERT_FILE, TYRO_KEY_FILE, TYRO_ACAO_HEADER")
		fmt.Println("If a certificate file is provided, Tyro will attempt to use HTTPS.")
		fmt.Println("The Access-Control-Allow-Origin header for CORS is only set for the /status/[bibID] endpoint.")
	}

	flag.Parse()

	//Look at the environment variables.
	fromEnv()

	logIfVerbose("Serving on address: " + *address)
	logIfVerbose("Using Client Key: " + *clientKey)
	logIfVerbose("Using Client Secret: " + *clientSecret)
	logIfVerbose("Connecting to API URL: " + *apiURL)

	if *headerACAO == "*" {
		fmt.Println("WARNING: USING \"*\" FOR \"Access-Control-Allow-Origin\" HEADER. API WILL BE PUBLIC!")
	}

	if *certFile != "" {
		logIfVerbose("Going to try to serve through HTTPS")
		logIfVerbose("Using Certificate File: " + *certFile)
		logIfVerbose("Using Private Key File: " + *keyFile)
	}

	tokenChan = make(chan string)
	refreshTokenChan = make(chan bool)
	go tokener()
	refreshTokenChan <- true

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/status/", statusHandler)
	rawProxy := httputil.NewSingleHostReverseProxy(&url.URL{})
	rawProxy.Director = rawRewriter
	http.Handle("/raw/", rawProxy)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	if *certFile == "" {
		log.Fatal(http.ListenAndServe(*address, nil))
	} else {
		//Remove SSL 3.0 compatibility for POODLE exploit mitigation
		config := &tls.Config{MinVersion: tls.VersionTLS10}
		server := &http.Server{Addr: *address, Handler: nil, TLSConfig: config}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	}

}

func homeHandler(w http.ResponseWriter, r *http.Request) {

	renderTemplate(w, "home", nil)

}

func statusHandler(w http.ResponseWriter, r *http.Request) {

	token := <-tokenChan

	if token == "uninitialized" {
		http.Error(w, "Token Error, token not yet created.", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, token not yet generated.")
		return
	}

	if token == "" {
		http.Error(w, "Token Error, token creation failed.", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, token creation failed.")
		return
	}

	bibID := strings.Split(r.URL.Path[len("/status/"):], "/")[0]

	if bibID == "" {
		http.Error(w, "Error, you need to provide a Bib ID. /status/[BidID]", http.StatusBadRequest)
		logIfVerbose("Bad Request at /status/ handler, no BidID provided.")
		return
	}

	parsedAPIURL, err := url.Parse(*apiURL)
	if err != nil {
		//No recovery possible here, probable problem with URL
		log.Fatal(err)
	}

	itemStatusURL := parsedAPIURL
	itemStatusURL.Path = path.Join(itemStatusURL.Path, ItemRequestEndpoint)

	q := itemStatusURL.Query()
	q.Set("bibIds", bibID)
	q.Set("deleted", "false")
	itemStatusURL.RawQuery = q.Encode()

	getItemStatus, err := http.NewRequest("GET", itemStatusURL.String(), nil)
	if err != nil {
		//No recovery possible here, probable problem with URL
		log.Fatal(err)
	}

	setAuthorizationHeaders(getItemStatus, r, token)

	client := &http.Client{}
	resp, err := client.Do(getItemStatus)
	if err != nil {
		http.Error(w, "Error querying Sierra API", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, GET against itemStatusURL failed.")
		logIfVerbose(err)
		return
	}

	if resp.StatusCode == 401 {
		http.Error(w, "Token is out of date, or is refreshing. Try request again.", http.StatusInternalServerError)
		logIfVerbose("Internal Server Error at /status/ handler, token is out of date.")
		refreshTokenChan <- true
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
		logIfVerbose("Internal Server Error at /status/ handler, JSON Decoding Error")
		logIfVerbose(err)
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

	json, err := json.MarshalIndent(statusJSON, "", "   ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	w.Header().Set("Content-Type", "application/json")
	w.Write(json)

}

func rawRewriter(r *http.Request) {

	token := <-tokenChan

	if token == "uninitialized" {
		logIfVerbose("Error at /raw/ handler, token not yet generated.")
		return
	}

	if token == "" {
		logIfVerbose("Error at /raw/ handler, token creation failed.")
		return
	}

	parsedAPIURL, err := url.Parse(*apiURL)
	if err != nil {
		//No recovery possible here, probable problem with URL
		log.Fatal(err)
	}

	rawRequestURL := parsedAPIURL
	rawRequestURL.Path = path.Join(rawRequestURL.Path, r.URL.Path[len("/raw/"):])
	rawRequestURL.RawQuery = r.URL.RawQuery

	r.URL = rawRequestURL

	setAuthorizationHeaders(r, r, token)

	logIfVerbose("Sending proxied request:")
	logIfVerbose(r)

}

func tokener() {

	type AuthTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	token := "uninitialized"

	for {
		select {
		case <-refreshTokenChan:

			logIfVerbose("Asking for new token...")

			stopIntrim := make(chan bool)

			go func() {
				logIfVerbose("Serving old token while we wait.")
				oldToken := token
			RunForever:
				for {
					select {
					case tokenChan <- oldToken:
						logIfVerbose("Sent token: " + oldToken)
					case <-stopIntrim:
						close(stopIntrim)
						break RunForever
					}
				}
			}()

			parsedAPIURL, err := url.Parse(*apiURL)
			if err != nil {
				//No recovery possible here, probable problem with URL
				log.Fatal(err)
			}

			tokenRequestURL := parsedAPIURL
			tokenRequestURL.Path = path.Join(tokenRequestURL.Path, TokenRequestEndpoint)

			bodyValues := url.Values{}
			bodyValues.Set("grant_type", "client_credentials")

			getTokenRequest, err := http.NewRequest("POST", tokenRequestURL.String(), bytes.NewBufferString(bodyValues.Encode()))
			if err != nil {
				//No recovery possible here, probable problem with URL
				log.Fatal(err)
			}

			getTokenRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			getTokenRequest.SetBasicAuth(*clientKey, *clientSecret)

			client := &http.Client{}
			resp, err := client.Do(getTokenRequest)
			if err != nil {
				token = ""
				logIfVerbose("Unable to get new token!")
				logIfVerbose(err)
				logIfVerbose(resp)
				return
			}

			var responseJSON AuthTokenResponse

			err = json.NewDecoder(resp.Body).Decode(&responseJSON)
			defer resp.Body.Close()
			if err != nil {
				token = ""
				logIfVerbose("Unable to parse new token response!")
				logIfVerbose(err)
				logIfVerbose(resp)
				return
			}

			logIfVerbose(responseJSON)

			stopIntrim <- true
			<-stopIntrim

			token = responseJSON.AccessToken

			logIfVerbose("Received new token from API.")

			go func() {
				time.Sleep(time.Duration(responseJSON.ExpiresIn-20) * time.Second)
				refreshTokenChan <- true
			}()

		case tokenChan <- token:
			logIfVerbose("Sent token: " + token)
		}
	}
}

//Utility Functions

func fromEnv() {
	getAddressFromEnvOrDefault()
	getVerboseFromEnvOrDefault()
	getURLFromEnvOrDefault()
	getCertFileFromEnvOrDefault()
	getKeyFileFromEnvOrDefault()
	getClientKeyFromEnvOrFail()
	getClientSecretFromEnvOrFail()
	getACAOHeaderFromEnvOrDefault()
}

//If the address is not set on the command line, get it from the
//environment or use the default.
func getAddressFromEnvOrDefault() {
	if *address == DefaultAddress {
		envAddress := os.Getenv(EnvPrefix + "ADDRESS")
		if envAddress != "" {
			*address = envAddress
		}
	}
}

//If verbose boolean is not set on the command line, get it from the
//environment or use the default.
func getVerboseFromEnvOrDefault() {
	if *verbose == DefaultVerbose {
		envVerboseString := os.Getenv(EnvPrefix + "VERBOSE")
		if envVerboseString != "" {
			envVerboseBool, err := strconv.ParseBool(envVerboseString)
			if err != nil {
				log.Fatalf("Unable to parse '%v' (%v) as boolean.", envVerboseString, EnvPrefix+"VERBOSE")
			} else {
				*verbose = envVerboseBool
			}
		}
	}
}

//If the API URL is not set on the command line, get it from the
//environment or use the default.
func getURLFromEnvOrDefault() {
	if *apiURL == DefaultURL {
		envURL := os.Getenv(EnvPrefix + "URL")
		if envURL != "" {
			*apiURL = envURL
		}
	}
}

//If the certification file is not set on the command line, get it
//from the environment or use the default.
func getCertFileFromEnvOrDefault() {
	if *certFile == "" {
		envCertFile := os.Getenv(EnvPrefix + "CERT_FILE")
		if envCertFile != "" {
			*certFile = envCertFile
		}
	}
}

//If the key file is not set on the command line, get it
//from the environment or use the default.
func getKeyFileFromEnvOrDefault() {
	if *keyFile == "" {
		envKeyFile := os.Getenv(EnvPrefix + "KEY_FILE")
		if envKeyFile != "" {
			*keyFile = envKeyFile
		}
	}
}

//If the client key is not set on the command line, get it
//from the environment or exit.
func getClientKeyFromEnvOrFail() {
	if *clientKey == "" {
		envClientKey := os.Getenv(EnvPrefix + "KEY")
		if envClientKey != "" {
			*clientKey = envClientKey
		} else {
			log.Fatalf("Unable to find Client Key in environment variable %v or provided by flag -key=",
				EnvPrefix+"KEY")
		}
	}
}

//If the client secret is not set on the command line, get it
//from the environment or exit.
func getClientSecretFromEnvOrFail() {
	if *clientSecret == "" {
		envClientSecret := os.Getenv(EnvPrefix + "SECRET")
		if envClientSecret != "" {
			*clientSecret = envClientSecret
		} else {
			log.Fatalf("Unable to find Client Secret in environment variable %v or provided by flag -secret=",
				EnvPrefix+"Secret")
		}
	}
}

//If the Access-Control-Allow-Origin header is not set on the command line, get it
//from the environment or use the default.
func getACAOHeaderFromEnvOrDefault() {
	if *headerACAO == DefaultACAOHeader {
		envHeaderACAO := os.Getenv(EnvPrefix + "ACAO_HEADER")
		if envHeaderACAO != "" {
			*headerACAO = envHeaderACAO
		}
	}
}

//Log a message if the verbose flag is set.
func logIfVerbose(message interface{}) {
	if *verbose {
		log.Println(message)
	}
}

//Render an HTML template.
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//Set the required Authorization headers.
//This includes the Bearer token, the user agent, and X-Forwarded-For
func setAuthorizationHeaders(nr *http.Request, or *http.Request, t string) {
	nr.Header.Add("Authorization", "Bearer "+t)
	nr.Header.Add("User-Agent", "Tyro")

	originalForwardFor := or.Header.Get("X-Forwarded-For")
	if originalForwardFor == "" {
		ip, _, _ := net.SplitHostPort(or.RemoteAddr)
		nr.Header.Add("X-Forwarded-For", ip)
	} else {
		nr.Header.Add("X-Forwarded-For", originalForwardFor)
	}
}
