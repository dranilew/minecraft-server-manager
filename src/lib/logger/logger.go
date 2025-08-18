// Package logger is a logging library for logging in the manager.
package logger

import (
	"flag"
	"io"
	"log"
	"os"
)

var (
	// loggers is the list of supported loggers.
	loggers []*log.Logger
	// Debug indicates whether to print Debug logs or not.
	Debug = flag.Bool("v", false, "Whether to log more than usual.")
)

func init() {
	flag.Parse()
}

// Init initializes the loggers.
func Init(tag string, extraLoggers ...io.Writer) error {
	return initPlatformLogger(tag, extraLoggers)
}

// Print prints to each of the loggers.
func Printf(message string, v ...any) {
	for _, logger := range loggers {
		logger.Printf(message, v...)
	}
}

// Fatal prints before exiting.
func Fatalf(message string, v ...any) {
	Printf(message, v...)
	os.Exit(1)
}

// Debug prints only if Debug is set.
func Debugf(message string, v ...any) {
	if *Debug {
		Printf(message, v...)
	}
}
