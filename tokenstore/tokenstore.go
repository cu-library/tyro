// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

//Package token stores the current sierra API access
//token in a safe container. Access is controlled
//by a sync.RWMutex
package tokenstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cudevmaxwell/tyro/loglevel"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const UninitialedTokenValue string = "uninitialized"

//The number of seconds before a token would expire
//a new token is asked for.
//For example, if a token would expire in 50 seconds,
//and the TokenRefreshBuffer is 5 seconds,
//ask for a new token in 45 seconds.
const TokenRefreshBuffer int = 5

//The number of seconds a refresh will be scheduled for
//in the event of an error.
//Also the amount of time a Get() will wait for the initial
//token
const DefaultRefreshTime int = 30

type LogMessage struct {
	Message interface{}
	Level   loglevel.LogLevel
}

type TokenStore struct {
	Lock        sync.RWMutex
	Value       string
	Refresh     chan struct{}
	LogMessages chan LogMessage
	Initialized  chan struct{}
}

func NewTokenStore() *TokenStore {
	t := new(TokenStore)
	t.Refresh = make(chan struct{})
	t.LogMessages = make(chan LogMessage, 100)
	t.Initialized = make(chan struct{})
	t.Value = UninitialedTokenValue

	return t
}

func (t *TokenStore) Get() (string, error) {
	t.Lock.RLock()
	defer t.Lock.RUnlock()
	if t.Value == "" {
		return "", errors.New("Token generation error.")
	}
	return t.Value, nil
}

func (t *TokenStore) set(nt string) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	if t.Value == UninitialedTokenValue {
		t.Initialized <- struct{}{}
	}
	t.Value = nt
}

//This is the function which will run forever, waiting for a timeout
//or a message on the Refresh channel. It will exit if the Refresh
//channel is closed.
func (t *TokenStore) Refresher(tokenURL *url.URL, clientKey, clientSecret *string) {

	runRefreshSetUpNext := func() <-chan time.Time {
		refreshIn, err := t.refresh(tokenURL, clientKey, clientSecret)
		if err != nil {
			t.LogMessages <- LogMessage{err, loglevel.ErrorMessage}
			refreshIn = DefaultRefreshTime
		}
		futureTime := refreshIn - TokenRefreshBuffer
		lm := fmt.Sprintf("%v seconds in the future, a refresh will happen.", futureTime)
		t.LogMessages <- LogMessage{lm, loglevel.TraceMessage}
		return time.After(time.Duration(refreshIn-TokenRefreshBuffer) * time.Second)
	}

	refreshOrTimeout := func(timeout <-chan time.Time) (<-chan time.Time, error) {
		select {
		case <-timeout:
			t.LogMessages <- LogMessage{"The old token timed out.", loglevel.TraceMessage}
			return runRefreshSetUpNext(), nil
		case _, ok := <-t.Refresh:
			if ok {
				t.LogMessages <- LogMessage{"A new token has been requested", loglevel.TraceMessage}
				return runRefreshSetUpNext(), nil
			} else {
				close(t.LogMessages)
				return make(<-chan time.Time), errors.New("Refresh channel is closed.")
			}
		}
	}

	go func() {
		toc := runRefreshSetUpNext()
		err := errors.New("")
		for {
			toc, err = refreshOrTimeout(toc)
			if err != nil {
				return
			}
		}
	}()

}

func (t *TokenStore) refresh(tokenURL *url.URL, clientKey, clientSecret *string) (int, error) {

	type AuthTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	time.Sleep(10 * time.Second)

	bodyValues := url.Values{}
	bodyValues.Set("grant_type", "client_credentials")
	getTokenRequest, err := http.NewRequest("POST", tokenURL.String(), bytes.NewBufferString(bodyValues.Encode()))
	if err != nil {
		t.set("")
		t.LogMessages <- LogMessage{err, loglevel.WarnMessage}
		return 0, err
	}
	getTokenRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	getTokenRequest.SetBasicAuth(*clientKey, *clientSecret)
	client := new(http.Client)
	resp, err := client.Do(getTokenRequest)
	if err != nil {
		t.set("")
		t.LogMessages <- LogMessage{err, loglevel.WarnMessage}
		return 0, err
	}
	if resp.StatusCode != 200 {
		t.set("")
		t.LogMessages <- LogMessage{err, loglevel.WarnMessage}
		return 0, errors.New("Unable to authenticate to token generator.")
	}

	var responseJSON AuthTokenResponse

	err = json.NewDecoder(resp.Body).Decode(&responseJSON)
	defer resp.Body.Close()

	if err != nil {
		t.set("")
		t.LogMessages <- LogMessage{err, loglevel.WarnMessage}
		return 0, err
	}

	if responseJSON.ExpiresIn < 10 {
		t.set("")
		return 0, errors.New("Token is set for too small a time.")
	} else {
		t.LogMessages <- LogMessage{"Recieved Token", loglevel.TraceMessage}
		t.set(responseJSON.AccessToken)
		return responseJSON.ExpiresIn, nil
	}
}
