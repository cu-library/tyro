// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/cudevmaxwell/tyro/loglevel"
	"github.com/cudevmaxwell/tyro/tokenstore"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

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

func TestStatusHandlerNoBibId(t *testing.T) {

	setupLogging()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(ts.URL, "", "")
	go func() {
		for _ = range tokenStore.LogMessages {
		}
	}()

	req, err := http.NewRequest("GET", "/status/", nil)
	if err != nil {
		t.Fatal(err)
	}

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
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(ts.URL, "", "")
	go func() {
		for _ = range tokenStore.LogMessages {
		}
	}()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"entries":[{"id":2536252,"updatedDate":"2014-09-19T03:09:16Z","createdDate":"2007-05-11T18:37:00Z","deleted":false,"bibIds":[2401597],"location":{"code":"flr4 ","name":"Floor 4 Books"},"status":{"code":"-","display":"IN LIBRARY"},"barcode":"12016135026","callNumber":"|aJC578.R383|bG67 2007"}]}`)
	}))
	defer ts2.Close()

	//Get StatusHandler to look at our mocked server
	oldAPIURL := *apiURL
	*apiURL = ts2.URL
	defer func() { *apiURL = oldAPIURL }()

	req, err := http.NewRequest("GET", "/status/2401597", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	statusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status handler didn't return %v when provided with a good response.", http.StatusBadRequest)
	}
}

func TestRawHandlerTestRewrite(t *testing.T) {

	setupLogging()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(ts.URL, "", "")
	go func() {
		for _ = range tokenStore.LogMessages {
		}
	}()

	req, err := http.NewRequest("GET", "/raw/?bibIds=1234", nil)
	if err != nil {
		t.Fatal(err)
	}

	oldAPIURL := *apiURL
	*apiURL = "http://apiurl.com/test/"
	defer func() { *apiURL = oldAPIURL }()

	rawRewriter(req)

	if req.URL.String() != "http://apiurl.com/test?bibIds=1234" {
		t.Error("The raw handler is not correctly rewriting the url")
	}

}

func TestParseURLandEndpoint(t *testing.T) {

	goodURL := "http://test.com"
	endpoint := "test"
	badURL := ":"

	parsedURL, err := parseURLandEndpoint(goodURL, endpoint)
	if err != nil {
		t.Error("The parse should not have failed.")
	}
	if parsedURL.String() != "http://test.com/test" {
		t.Error("Bad join")
	}

	parsedURL, err = parseURLandEndpoint(badURL, endpoint)
	if err == nil {
		t.Error("Parse should have failed")
	}

}

func TestSettingAuthorizationHeaders(t *testing.T) {

	//The default case

	oldRequest, _ := http.NewRequest("GET", "/test", nil)
	oldRequest.RemoteAddr = "7.7.7.7:8888"
	newRequest, _ := http.NewRequest("GET", "/test", nil)

	setAuthorizationHeaders(newRequest, oldRequest, "token")

	if newRequest.Header.Get("Authorization") != "Bearer token" {
		t.Error("The Authorization header is not being set properly.")
	}
	if newRequest.Header.Get("User-Agent") != "Tyro" {
		t.Error("The User-Agent header is not being set properly.")
	}
	if newRequest.Header.Get("X-Forwarded-For") != "7.7.7.7" {
		t.Error("The X-Forwarded-For header is not being set properly.")
	}

	//Badly formed remoteaddr

	oldRequest, _ = http.NewRequest("GET", "/test", nil)
	oldRequest.RemoteAddr = ":wef:wf:"
	newRequest, _ = http.NewRequest("GET", "/test", nil)

	setAuthorizationHeaders(newRequest, oldRequest, "token")

	if newRequest.Header.Get("X-Forwarded-For") != "" {
		t.Error("The X-Forwarded-For header is being set when it shouldn't be.")
	}

	//An X-Forwarded-For in the incoming request

	oldRequest, _ = http.NewRequest("GET", "/test", nil)
	oldRequest.Header.Add("X-Forwarded-For", "1.2.3.4")
	newRequest, _ = http.NewRequest("GET", "/test", nil)

	setAuthorizationHeaders(newRequest, oldRequest, "token")

	if newRequest.Header.Get("X-Forwarded-For") != "1.2.3.4" {
		t.Error("The X-Forwarded-For header is not being set properly.")
	}

}

func setupLogging() {
	LogMessageLevel = loglevel.ErrorMessage
	log.SetOutput(os.Stderr)
}
