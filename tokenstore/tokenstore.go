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
	l "github.com/cudevmaxwell/tyro/loglevel"
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

//If the API returns a TTL less than this, error out.
const MinimumTokenTTL int = 10

//The number of seconds a refresh will be scheduled for
//in the event of an error.
const DefaultRefreshTime int = 10

type TokenStore struct {
	lock        sync.RWMutex
	value       string
	Refresh     chan struct{}
	Initialized chan struct{}
}

func NewTokenStore() *TokenStore {
	t := new(TokenStore)
	t.Refresh = make(chan struct{})
	t.Initialized = make(chan struct{}, 1)
	t.value = UninitialedTokenValue

	return t
}

func (t *TokenStore) Get() (string, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.value == "" {
		return "", errors.New("Token generation error.")
	}
	l.Log("Sending token.", l.TraceMessage)
	return t.value, nil
}

func (t *TokenStore) set(nt string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.value == UninitialedTokenValue {
		t.Initialized <- struct{}{}
	}
	t.value = nt
}

//This function runs forever, waiting for a timeout
//or a message on the Refresh channel. It will exit if the Refresh
//channel is closed.
func (t *TokenStore) Refresher(tokenURL, clientKey, clientSecret string) {

	runRefreshSetUpNext := func() <-chan time.Time {
		refreshIn, err := t.refresh(tokenURL, clientKey, clientSecret)
		if err != nil {
			l.Log(err, l.ErrorMessage)
			refreshIn = DefaultRefreshTime + TokenRefreshBuffer
		}
		futureTime := refreshIn - TokenRefreshBuffer
		lm := fmt.Sprintf("%v seconds in the future, a refresh will happen.", futureTime)
		l.Log(lm, l.TraceMessage)
		return time.After(time.Duration(futureTime) * time.Second)
	}

	refreshOrTimeout := func(timeout <-chan time.Time) (<-chan time.Time, error) {
		select {
		case <-timeout:
			l.Log("The old token timed out.", l.TraceMessage)
			return runRefreshSetUpNext(), nil
		case _, ok := <-t.Refresh:
			if ok {
				l.Log("A new token has been requested", l.TraceMessage)
				return runRefreshSetUpNext(), nil
			} else {
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

func (t *TokenStore) refresh(tokenURL, clientKey, clientSecret string) (int, error) {

	type AuthTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	bodyValues := url.Values{}
	bodyValues.Set("grant_type", "client_credentials")
	getTokenRequest, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(bodyValues.Encode()))
	if err != nil {
		t.set("")
		l.Log(err, l.WarnMessage)
		return 0, err
	}
	getTokenRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	getTokenRequest.SetBasicAuth(clientKey, clientSecret)
        getTokenRequest.Close = true
	client := new(http.Client)
	resp, err := client.Do(getTokenRequest)
	if err != nil {
		t.set("")
		l.Log(err, l.WarnMessage)
		return 0, err
	}
	if resp.StatusCode != 200 {
		t.set("")
		l.Log(err, l.WarnMessage)
		return 0, fmt.Errorf("Unable to authenticate to token generator, %v", resp.StatusCode)
	}

	var responseJSON AuthTokenResponse

	err = json.NewDecoder(resp.Body).Decode(&responseJSON)
	defer resp.Body.Close()

	if err != nil {
		t.set("")
		l.Log(err, l.WarnMessage)
		return 0, err
	}

	if responseJSON.ExpiresIn < MinimumTokenTTL {
		t.set("")
		return 0, errors.New("Token has a expire_in that is too small.")
	} else {
		l.Log("Received Token", l.TraceMessage)
		t.set(responseJSON.AccessToken)
		return responseJSON.ExpiresIn, nil
	}
}
