// Copyright 2014 Kevin Bowrin All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package loglevel

import (
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
