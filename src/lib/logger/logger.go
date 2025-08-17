// Package logger is a logging library for logging in the manager.
package logger

import (
	"flag"
	"log"
	"os"
)

var (
	// loggers is the list of supported loggers.
	loggers []*log.Logger
	// debug indicates whether to print debug logs or not.
	debug = flag.Bool("v", false, "Whether to log more than usual.")
)

func init() {
	flag.Parse()
}

// Init initializes the loggers.
func Init(tag string) error {
	return initPlatformLogger(tag)
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
	if *debug {
		Printf(message, v...)
	}
}
