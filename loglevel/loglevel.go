package loglevel

import (
	"gopkg.in/cudevmaxwell-vendor/lumberjack.v2"
	"log"
	"os"
	"sync"
)

type LogLevel int

const (
	//Logging
	DefaultLogFileLocation string = "stdout"
	DefaultLogMaxSize      int    = 100
	DefaultLogMaxBackups   int    = 0
	DefaultLogMaxAge       int    = 0

	ErrorMessage LogLevel = iota
	WarnMessage
	InfoMessage
	DebugMessage
	TraceMessage
)

var logMessageLevel = ErrorMessage
var logMessageLevelMutex = new(sync.RWMutex)

func Set(l LogLevel) {
	logMessageLevelMutex.Lock()
	defer logMessageLevelMutex.Unlock()

	logMessageLevel = l
}

//Log a message if the level is below or equal to the set LogMessageLevel
func Log(message interface{}, messagelevel LogLevel) {
	logMessageLevelMutex.RLock()
	defer logMessageLevelMutex.RUnlock()

	if messagelevel <= logMessageLevel {
		log.Printf("%v: %v\n", messagelevel, message)
	}
}

func (l LogLevel) String() string {
	switch l {
	case ErrorMessage:
		return "ERROR"
	case WarnMessage:
		return "WARN"
	case InfoMessage:
		return "INFO"
	case DebugMessage:
		return "DEBUG"
	case TraceMessage:
		return "TRACE"
	}
	return "UNKNOWN"
}

//TODO: The LogLevel type already has a String() function. Use that.
func ParseLogLevel(logLevel string) LogLevel {
	switch logLevel {
	case "error":
		return ErrorMessage
	case "warn":
		return WarnMessage
	case "info":
		return InfoMessage
	case "debug":
		return DebugMessage
	case "trace":
		return TraceMessage
	}
	return TraceMessage
}

func SetupLumberjack(logFileLocation string, logMaxSize, logMaxBackups, logMaxAge int) {
	if logFileLocation != DefaultLogFileLocation {
		lj := &lumberjack.Logger{
			Filename:   logFileLocation,
			MaxSize:    logMaxSize,
			MaxBackups: logMaxBackups,
			MaxAge:     logMaxAge,
		}
		if _, err := lj.Write([]byte("Stating...\n")); err != nil {
			log.Fatalf("Unable to open logfile %v", logFileLocation)
		} else {
			log.SetOutput(lj)
		}
	} else {
		log.SetOutput(os.Stdout)
	}
}
