// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	l "github.com/cudevmaxwell/tyro/loglevel"
	"github.com/cudevmaxwell/tyro/tokenstore"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() {
	l.Set(l.ErrorMessage)
	log.SetOutput(ioutil.Discard)
}

func TestHomeHandler(t *testing.T) {

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

func TestStatusHandler(t *testing.T) {

	req, err := http.NewRequest("GET", "/status/", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	statusHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status handler didn't return %v.", http.StatusBadRequest)
	}
}

func TestStatusBibHandlerNoBibId(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(ts.URL, "", "")
	defer close(tokenStore.Refresh)

	req, err := http.NewRequest("GET", "/status/bib/", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	statusBibHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status handler didn't error %v when no bib id provided", http.StatusBadRequest)
	}

	if w.Body.String() != "Error, you need to provide a BibID. /status/bib/[BidID]\n" {
		t.Error("Status handler didn't return the correct information when no bib id provided")
	}

}

func TestStatusBibHandlerGoodResponseFromSierra(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(ts.URL, "", "")
	defer close(tokenStore.Refresh)

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"entries":[{"id":2536252,"updatedDate":"2014-09-19T03:09:16Z","createdDate":"2007-05-11T18:37:00Z","deleted":false,"bibIds":[2401597],"location":{"code":"flr4 ","name":"Floor 4 Books"},"status":{"code":"-","display":"IN LIBRARY"},"barcode":"12016135026","callNumber":"|aJC578.R383|bG67 2007"}]}`)
	}))
	defer ts2.Close()

	//Get statusBibIDHandler to look at our mocked server
	oldAPIURL := *apiURL
	*apiURL = ts2.URL
	defer func() { *apiURL = oldAPIURL }()

	req, err := http.NewRequest("GET", "/status/bib/2401597", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	statusBibHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status handler didn't return %v when provided with a good response.", http.StatusBadRequest)
	}
}

func TestRawHandlerTestRewrite(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(ts.URL, "", "")
	defer close(tokenStore.Refresh)

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

func TestParseURLandJoinToPath(t *testing.T) {

	goodURL := "http://test.com"
	endpoint := "test"
	badURL := ":"

	parsedURL, err := parseURLandJoinToPath(goodURL, endpoint)
	if err != nil {
		t.Error("The parse should not have failed.")
	}
	if parsedURL.String() != "http://test.com/test" {
		t.Error("Bad join")
	}

	parsedURL, err = parseURLandJoinToPath(badURL, endpoint)
	if err == nil {
		t.Error("Parse should have failed")
	}

}

func TestGetTokenOrErrorFailTokenStoreInitialized(t *testing.T) {

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(":", "", "")
	defer close(tokenStore.Refresh)

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	<-tokenStore.Initialized

	if _, err := getTokenOrError(w, r); err == nil {
		t.Error("Should have failed with unparseable URL.")
	}

	if w.Code != http.StatusInternalServerError {
		t.Error("Should have returned a InternalServerError")
	}

}

func TestGetTokenOrErrorFailTokenStoreUninitialized(t *testing.T) {

	tokenStore = tokenstore.NewTokenStore()
	tokenStore.Refresher(":", "", "")
	defer close(tokenStore.Refresh)

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	if _, err := getTokenOrError(w, r); err == nil {
		t.Error("Should have failed with unparseable URL.")
	}

	if w.Code != http.StatusInternalServerError {
		t.Error("Should have returned a InternalServerError")
	}

}

func TestGetTokenOrErrorFailTokenStoreTimeout(t *testing.T) {

	tokenStore = tokenstore.NewTokenStore()

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	time.Sleep(time.Second * 31)

	if _, err := getTokenOrError(w, r); err == nil {
		t.Error("Should have failed with unparseable URL.")
	}

	if w.Code != http.StatusInternalServerError {
		t.Error("Should have returned a InternalServerError")
	}

}

//The default case. Don't set the header at all.
func TestSetACAOHeaderNoConfig(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	setACAOHeader(w, r, "")
	if w.HeaderMap.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Access-Control-Allow-Origin shouldn't be set.")
	}
}

//Set the header to *.
func TestSetACAOHeaderAllOrigins(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	setACAOHeader(w, r, "*")
	if w.HeaderMap.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Access-Control-Allow-Origin not set properly.")
	}
}

//Set the ACAO config to a single origin which doesn't match.
func TestSetACAOHeaderNotMatchOnSingle(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	setACAOHeader(w, r, "http://test.com")
	if w.HeaderMap.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Access-Control-Allow-Origin shouldn't be set.")
	}
}

//Set the ACAO config to a single origin which does match.
func TestSetACAOHeaderMatchOnSingle(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", "http://test.com")
	w := httptest.NewRecorder()
	setACAOHeader(w, r, "http://test.com")
	if w.HeaderMap.Get("Access-Control-Allow-Origin") != "http://test.com" {
		t.Error("Access-Control-Allow-Origin not set properly.")
	}
}

//Set the ACAO config to a one of a list of origins, none of which match.
func TestSetACAOHeaderNoMatchOnList(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", "http://test3.com")
	w := httptest.NewRecorder()
	setACAOHeader(w, r, "http://test.com;http://test2.com;")
	if w.HeaderMap.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Access-Control-Allow-Origin shouldn't be set.")
	}
}

//Set the ACAO config to a one of a list of origins, one of which does match.
func TestSetACAOHeaderMatchOnList(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", "http://test2.com")
	w := httptest.NewRecorder()
	setACAOHeader(w, r, "http://test.com;http://test2.com;")
	if w.HeaderMap.Get("Access-Control-Allow-Origin") != "http://test2.com" {
		t.Error("Access-Control-Allow-Origin not set properly.")
	}
}
