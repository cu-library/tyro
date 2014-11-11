// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package sierraapi

import (
	"net/http"
	"testing"
)

func TestSettingAuthorizationHeaders(t *testing.T) {

	//The default case
	oldRequest, _ := http.NewRequest("GET", "/test", nil)
	oldRequest.RemoteAddr = "7.7.7.7:8888"
	newRequest, _ := http.NewRequest("GET", "/test", nil)

	SetAuthorizationHeaders(newRequest, oldRequest, "token")

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

	SetAuthorizationHeaders(newRequest, oldRequest, "token")

	if newRequest.Header.Get("X-Forwarded-For") != "" {
		t.Error("The X-Forwarded-For header is being set when it shouldn't be.")
	}

	//An X-Forwarded-For in the incoming request

	oldRequest, _ = http.NewRequest("GET", "/test", nil)
	oldRequest.Header.Add("X-Forwarded-For", "1.2.3.4")
	newRequest, _ = http.NewRequest("GET", "/test", nil)

	SetAuthorizationHeaders(newRequest, oldRequest, "token")

	if newRequest.Header.Get("X-Forwarded-For") != "1.2.3.4" {
		t.Error("The X-Forwarded-For header is not being set properly.")
	}

}
