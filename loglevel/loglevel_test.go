// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package loglevel

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogLevelParse(t *testing.T) {

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
		if level := ParseLogLevel(k); level != v {
			t.Errorf("Unable to parse log level string %v properly", k)
		}
	}

	if level := ParseLogLevel("blahblahblah"); level != TraceMessage {
		t.Error("Default case for string to log level broken.")
	}

}

func TestLogLevel(t *testing.T) {

	logLevelToExpectedLength := map[LogLevel]int{
		ErrorMessage: 2,
		WarnMessage:  3,
		InfoMessage:  4,
		DebugMessage: 5,
		TraceMessage: 6,
	} //One more than expected, because of empty string at end of Split()

	logLevels := []LogLevel{
		ErrorMessage,
		WarnMessage,
		InfoMessage,
		DebugMessage,
		TraceMessage,
	}

	for _, level := range logLevels {
		b := new(bytes.Buffer)
		Set(level)
		for _, messageLevel := range logLevels {
			log.SetOutput(b)
			Log("x", messageLevel)
		}
		if len(strings.Split(b.String(), "\n")) != logLevelToExpectedLength[level] {
			t.Logf("%#v", strings.Split(b.String(), "\n"))
			t.Errorf("The log level %v logged the wrong number of messages.", level)
		}
	}

}