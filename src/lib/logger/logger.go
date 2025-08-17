// Package logger is a logging library for logging in the manager.
package logger

import (
	"log"
	"os"
)

var (
	// loggers is the list of supported loggers.
	loggers []*log.Logger
	// stderrLogger is for debugging.
	stderrLogger = log.New(os.Stderr, "", 0)
)

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

// Debug prints to the Stderr logger.
func Debugf(message string, v ...any) {
	stderrLogger.Printf(message, v...)
}
