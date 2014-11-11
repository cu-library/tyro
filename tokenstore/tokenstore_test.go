// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package tokenstore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenSetAndGet(t *testing.T) {

	tok := NewTokenStore()

	tokenVal, err := tok.Get()
	if err != nil {
		t.Error("Token Get() should not have returned an error before initial Set().")
	}
	if tokenVal != UninitialedTokenValue {
		t.Error("Token value should be UninitialedTokenValue before initial Set().")
	}
	go tok.set("token")
	select {
	case <-tok.Initialized:
		tokenVal, err := tok.Get()
		if err != err {
			t.Error("Token Get() should not have returned an error after correct Set().")
		}
		if tokenVal != "token" {
			t.Error("Token not set to the correct value.")
		}
	case <-time.After(time.Second * 1):
		t.Error("Initialized channel should have sent by now.")
	}

	tok.set("")
	tokenVal, err = tok.Get()
	if err == nil {
		t.Error("Token Get() should have returned an error after set to empty string.")
	}
}

func TestTokenRefresh(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":3600}`)
	}))
	defer ts.Close()

	tok := NewTokenStore()

	refresh, err := tok.refresh(ts.URL, "", "")
	if err != nil {
		t.Error("Token refresh() should have worked.")
	}
	if refresh != 3600 {
		t.Error("Token refresh() didn't return the right timeout.")
	}

	tokenVal, err := tok.Get()
	if err != nil {
		t.Error("Token Get() should not have returned an error.")
	}
	if tokenVal != "test" {
		t.Error("Token refresh() didn't return the right value.")
	}
}

func TestTokenRefreshFailBadParse(t *testing.T) {

	tok := NewTokenStore()

	_, err := tok.refresh(":", "", "")
	if err == nil {
		t.Error("Token refresh() should not have worked with nonsense tokenURL")
	}
	_, err = tok.Get()
	if err == nil {
		t.Error("Get should have failed with nonsense URL")
	}
}

func TestTokenRefreshFailBadClientDo(t *testing.T) {

	tok := NewTokenStore()

	_, err := tok.refresh("@#J#*FHQA@J@(FFU(#R@#NR@#(RAU(A*CC*##(#", "", "")
	if err == nil {
		t.Error("Token refresh() should not have worked with nonsense tokenURL")
	}
	_, err = tok.Get()
	if err == nil {
		t.Error("Get should have failed with nonsense URL")
	}
}

func TestTokenRefreshFailAuthentication(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, `{"error":"bad token"}`)
	}))
	defer ts.Close()

	tok := NewTokenStore()

	_, err := tok.refresh(ts.URL, "", "")
	if err == nil {
		t.Error("Token refresh() should not have worked with StatusNotFound on")
	}
	_, err = tok.Get()
	if err == nil {
		t.Error("Get should have failed with StatusNotFound return")
	}
}

func TestTokenRefreshFailBadJSON(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `BLAHBLAHBLAH{}{}BLAHBLAHBLAH`)
	}))
	defer ts.Close()

	tok := NewTokenStore()

	_, err := tok.refresh(ts.URL, "", "")
	if err == nil {
		t.Error("Token refresh() should not have worked with nonsense JSON.")
	}

	_, err = tok.Get()
	if err == nil {
		t.Error("Get should have failed with nonsense JSON")
	}
}

func TestTokenRefreshFailShortTTL(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"access_token":"test","token_type":"bearer","expires_in":1}`)
	}))
	defer ts.Close()

	tok := NewTokenStore()
	_, err := tok.refresh(ts.URL, "", "")
	if err == nil {
		t.Error("Token refresh() should not have worked with really small TTL.")
	}
	_, err = tok.Get()
	if err == nil {
		t.Error("Get should have failed with really small TTL")
	}
}

func TestRefresherTimeout(t *testing.T) {

	ran := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		if ran == false {
			t.Log("Go first")
			fmt.Fprintln(w, `{"access_token":"firsttoken","token_type":"bearer","expires_in":10}`)
		} else {
			t.Log("Go second")
			fmt.Fprintln(w, `{"access_token":"secondtoken","token_type":"bearer","expires_in":3600}`)
		}
		ran = true
	}))
	defer ts.Close()

	tok := NewTokenStore()

	tok.Refresher(ts.URL, "", "")
	defer close(tok.Refresh)

	token, err := tok.Get()
	if err != nil {
		t.Error("Get should not have failed before initial value assigned.")
	}
	if token != UninitialedTokenValue {
		t.Error("Unexpected token value")
	}

	<-tok.Initialized

	token, err = tok.Get()
	if err != nil {
		t.Error("Get should not have failed after initial value assigned.")
	}
	if token != "firsttoken" {
		t.Error("Unexpected token value")
	}

	time.Sleep(12 * time.Second)

	token, err = tok.Get()
	if err != nil {
		t.Error("Get should not have failed after next value assigned.")
	}
	if token != "secondtoken" {
		t.Error("Unexpected token value")
		t.Log(token)
	}

}

func TestRefresherRequestNew(t *testing.T) {

	ran := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ran == false {
			t.Log("Go first")
			fmt.Fprintln(w, `{"access_token":"firsttoken","token_type":"bearer","expires_in":3600}`)
		} else {
			t.Log("Go second")
			fmt.Fprintln(w, `{"access_token":"secondtoken","token_type":"bearer","expires_in":3600}`)
		}
		ran = true
	}))
	defer ts.Close()

	tok := NewTokenStore()
	tok.Refresher(ts.URL, "", "")
	defer close(tok.Refresh)

	<-tok.Initialized

	token, err := tok.Get()
	if err != nil {
		t.Error("Get should not have failed after initial value assigned.")
	}
	if token != "firsttoken" {
		t.Error("Unexpected token value")
	}

	tok.Refresh <- struct{}{}

	time.Sleep(1 * time.Millisecond)

	token, err = tok.Get()
	if err != nil {
		t.Error("Get should not have failed after next value assigned.")
	}
	if token != "secondtoken" {
		t.Error("Unexpected token value")
		t.Log(token)
	}

}

func TestRefresherRequestError(t *testing.T) {

	ran := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ran == false {
			t.Log("Go first")
			fmt.Fprintln(w, `{"access_token":"firsttoken","token_type":"bearer","expires_in":2}`)
		} else {
			t.Log("Go second")
			fmt.Fprintln(w, `{"access_token":"secondtoken","token_type":"bearer","expires_in":3600}`)
		}
		ran = true
	}))
	defer ts.Close()

	tok := NewTokenStore()
	tok.Refresher(ts.URL, "", "")
	defer close(tok.Refresh)

	<-tok.Initialized

	_, err := tok.Get()
	if err == nil {
		t.Error("Get should have failed after initial value assigned.")
	}

	time.Sleep(time.Duration(DefaultRefreshTime) * time.Second)

	token, err := tok.Get()
	if err != nil {
		t.Error("Get should not have failed after next value assigned.")
	}
	if token != "secondtoken" {
		t.Error("Unexpected token value")
		t.Log(token)
	}

}
