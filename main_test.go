// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
    "time"
)

/*
  _   _                        _   _                 _ _
 | | | | ___  _ __ ___   ___  | | | | __ _ _ __   __| | | ___ _ __
 | |_| |/ _ \| '_ ` _ \ / _ \ | |_| |/ _` | '_ \ / _` | |/ _ \ '__|
 |  _  | (_) | | | | | |  __/ |  _  | (_| | | | | (_| | |  __/ |
 |_| |_|\___/|_| |_| |_|\___| |_| |_|\__,_|_| |_|\__,_|_|\___|_|

*/

func TestHomeHandler(t *testing.T) {

	setupLogging()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	homeHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Home handler didn't return %v", http.StatusOK)
	}
}

func TestHomeHandler404(t *testing.T) {

	setupLogging()

	req, err := http.NewRequest("GET", "/badurlnocookie", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	homeHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Home handler didn't return %v for url which should not exist.", http.StatusNotFound)
	}
}

/*
  ____  _        _               _   _                 _ _
 / ___|| |_ __ _| |_ _   _ ___  | | | | __ _ _ __   __| | | ___ _ __
 \___ \| __/ _` | __| | | / __| | |_| |/ _` | '_ \ / _` | |/ _ \ '__|
  ___) | || (_| | |_| |_| \__ \ |  _  | (_| | | | | (_| | |  __/ |
 |____/ \__\__,_|\__|\__,_|___/ |_| |_|\__,_|_| |_|\__,_|_|\___|_|

*/

func TestStatusHandlerErrorUninitialized(t *testing.T) {

	setupLogging()

	req, err := http.NewRequest("GET", "/status/123", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		tokenChan <- UninitializedToken
	}()

	w := httptest.NewRecorder()
	statusHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status handler didn't error %v when token == uninitialized", http.StatusInternalServerError)
	}

	if w.Body.String() != "Token Error, token not yet created.\n" {
		t.Error("Status handler didn't return the correct information when token == uninitialized")
	}
}

func TestStatusHandlerErrorTokenEmpty(t *testing.T) {

	setupLogging()

	req, err := http.NewRequest("GET", "/status/123", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		tokenChan <- ErrorToken
	}()

	w := httptest.NewRecorder()
	statusHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status handler didn't error %v when token == \"\"", http.StatusInternalServerError)
	}

	if w.Body.String() != "Token Error, token creation failed.\n" {
		t.Error("Status handler didn't return the correct information when token == \"\" ")
	}
}

func TestStatusHandlerNoBibId(t *testing.T) {

	setupLogging()

	req, err := http.NewRequest("GET", "/status/", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		tokenChan <- "token"
	}()

	w := httptest.NewRecorder()
	statusHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status handler didn't error %v when no bib id provided", http.StatusBadRequest)
	}

	if w.Body.String() != "Error, you need to provide a Bib ID. /status/[BidID]\n" {
		t.Error("Status handler didn't return the correct information when no bib id provided")
	}

}

func TestStatusHandlerGoodResponseFromSierra(t *testing.T) {

	setupLogging()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"entries":[{"id":2536252,"updatedDate":"2014-09-19T03:09:16Z","createdDate":"2007-05-11T18:37:00Z","deleted":false,"bibIds":[2401597],"location":{"code":"flr4 ","name":"Floor 4 Books"},"status":{"code":"-","display":"IN LIBRARY"},"barcode":"12016135026","callNumber":"|aJC578.R383|bG67 2007"}]}`)
	}))
	defer ts.Close()

	//Get StatusHandler to look at our mocked server
	oldAPIURL := *apiURL
	*apiURL = ts.URL
	defer func() { *apiURL = oldAPIURL }()

	req, err := http.NewRequest("GET", "/status/2401597", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		tokenChan <- "token"
	}()

	w := httptest.NewRecorder()
	statusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status handler didn't return %v when provided with a good response.", http.StatusBadRequest)
	}
}

/*
  ____                  _   _                 _ _
 |  _ \ __ ___      __ | | | | __ _ _ __   __| | | ___ _ __
 | |_) / _` \ \ /\ / / | |_| |/ _` | '_ \ / _` | |/ _ \ '__|
 |  _ < (_| |\ V  V /  |  _  | (_| | | | | (_| | |  __/ |
 |_| \_\__,_| \_/\_/   |_| |_|\__,_|_| |_|\__,_|_|\___|_|

*/

func TestRawHandlerTestRewrite(t *testing.T) {

	setupLogging()

	req, err := http.NewRequest("GET", "/raw/?bibIds=1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		tokenChan <- "token"
	}()

	oldAPIURL := *apiURL
	*apiURL = "http://apiurl.com/test/"
	defer func() { *apiURL = oldAPIURL }()

	rawRewriter(req)

	if req.URL.String() != "http://apiurl.com/test?bibIds=1234" {
		t.Error("The raw handler is not correctly rewriting the url")
	}

}

/*
  _____     _                ____  _
 |_   _|__ | | _____ _ __   / ___|| |_ ___  _ __ __ _  __ _  ___
   | |/ _ \| |/ / _ \ '_ \  \___ \| __/ _ \| '__/ _` |/ _` |/ _ \
   | | (_) |   <  __/ | | |  ___) | || (_) | | | (_| | (_| |  __/
   |_|\___/|_|\_\___|_| |_| |____/ \__\___/|_|  \__,_|\__, |\___|
                                                      |___/
*/

func TestTokenStorage(t *testing.T) {

	setupLogging()  

    newServerRan := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        newServerRan = true
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

    oldAPIURL := *apiURL
    *apiURL = ts.URL
    defer func() { *apiURL = oldAPIURL }()

    defer func() {  
        tokenChan = make(chan string)
        refreshTokenChan = make(chan bool)
    }()

    go tokener()
    refreshTokenChan <- true

    for {
        if newServerRan == false{
            if token := <-tokenChan; token != UninitializedToken{
                t.Log(token)
                close(refreshTokenChan)
                t.Error("Token Storage not returning uninitialized token value before getting token from sierra api.")
            }
        } else {
            time.Sleep(5 * time.Second)
            if token := <-tokenChan; token != "test"{
                t.Log(token)
                close(refreshTokenChan)
                t.Error("Token Storage not returning the correct token value after getting token from sierra api.")
            }
            break 
        }
    }
}

/*
   ____             __ _                       _   _
  / ___|___  _ __  / _(_) __ _ _   _ _ __ __ _| |_(_) ___  _ __
 | |   / _ \| '_ \| |_| |/ _` | | | | '__/ _` | __| |/ _ \| '_ \
 | |__| (_) | | | |  _| | (_| | |_| | | | (_| | |_| | (_) | | | |
  \____\___/|_| |_|_| |_|\__, |\__,_|_|  \__,_|\__|_|\___/|_| |_|
                         |___/

*/

func TestLogLevelParse(t *testing.T) {

	setupLogging()

	var UnknownBadDumbLogLevel LogLevel = 9999

	//Our expected results, in maps
	logLevelToString := map[LogLevel]string{
		ErrorMessage:           "error",
		WarnMessage:            "warn",
		InfoMessage:            "info",
		DebugMessage:           "debug",
		TraceMessage:           "trace",
		UnknownBadDumbLogLevel: "unknown",
	}

	stringToLogLevel := map[string]LogLevel{
		"error": ErrorMessage,
		"warn":  WarnMessage,
		"info":  InfoMessage,
		"debug": DebugMessage,
		"trace": TraceMessage,
	}

	for k, v := range logLevelToString {

		if strings.ToLower(k.String()) != v {
			t.Errorf("Unable to parse log level %v properly", k)
		}

	}

	for k, v := range stringToLogLevel {

		if level := parseLogLevel(k); level != v {
			t.Errorf("Unable to parse log level string %v properly", k)
		}

	}

	if level := parseLogLevel("blahblahblah"); level != TraceMessage {
		t.Error("Default case for string to log level broken.")
	}

}

/*
  ____       _
 / ___|  ___| |_ _   _ _ __
 \___ \ / _ \ __| | | | '_ \
  ___) |  __/ |_| |_| | |_) |
 |____/ \___|\__|\__,_| .__/
                      |_|
*/

func setupLogging() {
	LogMessageLevel = ErrorMessage
	log.SetOutput(os.Stderr)
}
