// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package tokenstore

import (
    "testing"
    "time"
)

func TestTokenSetAndGet(t *testing.T) {

    tok := NewTokenStore() 

    tokenVal, err := tok.Get()
    if err != nil{
        t.Error("Token Get() should not have returned an error before initial Set().")
    } 
    if tokenVal != UninitialedTokenValue {
        t.Error("Token value should be UninitialedTokenValue before initial Set().")
    }
    go tok.set("token")
    select {
        case <-tok.Initialized:
            tokenVal, err := tok.Get()
            if err != err{
                t.Error("Token Get() should not have returned an error after correct Set().")
            } 
            if tokenVal != "token"{
                t.Error("Token not set to the correct value.")
            }
        case <-time.After(time.Second * 1):
            t.Error("Initialized channel should have sent by now.")
    }

    tok.set("")
    tokenVal, err = tok.Get()
    if err == nil{
        t.Error("Token Get() should have returned an error after set to empty string.")
    }       
}

