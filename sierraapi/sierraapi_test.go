// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package sierraapi

import (
	"bytes"
	"fmt"
	l "github.com/cudevmaxwell/tyro/loglevel"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestItemRecordConvert(t *testing.T) {

	//An example with an empty status
	exampleIn := ItemRecordIn{
		CallNumber: "|aJC578.R383|bG67 2007",
		Status: struct {
			DueDate time.Time "json:\"duedate\""
		}{DueDate: time.Time{}},
		Location: struct {
			Name string "json:\"name\""
		}{Name: "Floor 4 Books"},
	}

	exampleOut := ItemRecordOut{
		CallNumber: "JC578.R383 G67 2007",
		Status:     "In Library",
		Location:   "Floor 4 Books",
	}

	if *exampleIn.Convert() != exampleOut {
		t.Error("Expected the two examples to match after conversion.")
	}

	//An example with a due date
	due, _ := time.Parse(time.RFC3339, "2014-11-13T09:00:00Z")
	exampleIn = ItemRecordIn{
		CallNumber: "|aPR6068.O93|bH372 1999   ",
		Status: struct {
			DueDate time.Time "json:\"duedate\""
		}{DueDate: due},
		Location: struct {
			Name string "json:\"name\""
		}{Name: "Floor 3 Books"},
	}

	exampleOut = ItemRecordOut{
		CallNumber: "PR6068.O93 H372 1999",
		Status:     "Due November 13, 2014",
		Location:   "Floor 3 Books",
	}

	if *exampleIn.Convert() != exampleOut {
		t.Error("Expected the two examples to match after conversion.")
	}

}

func TestItemRecordsConvert(t *testing.T) {

	due, _ := time.Parse(time.RFC3339, "2014-11-13T09:00:00Z")

	exampleIn := ItemRecordsIn{
		Entries: []ItemRecordIn{
			ItemRecordIn{
				CallNumber: "|aJC578.R383|bG67 2007",
				Status: struct {
					DueDate time.Time "json:\"duedate\""
				}{DueDate: time.Time{}},
				Location: struct {
					Name string "json:\"name\""
				}{Name: "Floor 4 Books"},
			},
			ItemRecordIn{
				CallNumber: "|aPR6068.O93|bH372 1999   ",
				Status: struct {
					DueDate time.Time "json:\"duedate\""
				}{DueDate: due},
				Location: struct {
					Name string "json:\"name\""
				}{Name: "Floor 3 Books"},
			},
		},
	}

	exampleOut := ItemRecordsOut{
		Entries: []ItemRecordOut{
			ItemRecordOut{
				CallNumber: "JC578.R383 G67 2007",
				Status:     "In Library",
				Location:   "Floor 4 Books",
			},
			ItemRecordOut{
				CallNumber: "PR6068.O93 H372 1999",
				Status:     "Due November 13, 2014",
				Location:   "Floor 3 Books",
			},
		},
	}

	if !reflect.DeepEqual(*exampleIn.Convert(), exampleOut) {
		t.Error("Expected the two examples to match after conversion.")
	}

}

func TestSendRequestToAPIFailNewRequest(t *testing.T) {

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	if _, err := SendRequestToAPI(":", "", w, r); err == nil {
		t.Error("Should have failed with bad URL")
	}
	if w.Code != http.StatusInternalServerError {
		t.Error("Should have returned an InternalServerError with bad URL")
	}
}

func TestSendRequestToAPIFailBadRemoteAddrAndClientDo(t *testing.T) {

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.RemoteAddr = ":wef:wf:"
	w := httptest.NewRecorder()

	b := new(bytes.Buffer)
	log.SetOutput(b)
	defer log.SetOutput(os.Stderr)

	l.Set(l.WarnMessage)
	defer l.Set(l.ErrorMessage)

	if _, err := SendRequestToAPI("@#J#*FHQA@J@(FFU(#R@#NR@#(RAU(A*CC*##(#", "", w, r); err == nil {
		t.Error("Should have failed with nonsense URL")
	}
	if w.Code != http.StatusInternalServerError {
		t.Error("Should have returned an InternalServerError with nonsense URL")
	}

	if !strings.Contains(b.String(), "The remote address in an incoming request is not set properly.") {
		t.Error("Didn't log the bad remote addr.")
	}

}

func TestSendRequestToAPISuccess(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "test")
	}))
	defer ts.Close()

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	resp, err := SendRequestToAPI(ts.URL, "", w, r)
	if err != nil {
		t.Error("Didn't expect a fail on a good request.")
	}

	b := new(bytes.Buffer)
	b.ReadFrom(resp.Body)
	defer resp.Body.Close()
	if b.String() != "test\n" {
		t.Error("Expected to get back the correct body.")
	}
}
