// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package tokenstore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	exampleString := "dingding"
	clientKey := &exampleString
	clientSecret := &exampleString

	tok := NewTokenStore()

	u, _ := url.Parse(ts.URL)

	refresh, err := tok.refresh(u, clientKey, clientSecret)
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
