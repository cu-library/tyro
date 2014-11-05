package loglevel

type LogLevel int

const (
	ErrorMessage LogLevel = iota
	WarnMessage
	InfoMessage
	DebugMessage
	TraceMessage
)

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
