//go:build linux

package logger

import (
	"log"
	"log/syslog"
	"os"
)

func initPlatformLogger(tag string) error {
	syslogWriter, err := syslog.New(syslog.LOG_INFO, tag)
	if err != nil {
		return err
	}
	syslogLogger := log.New(syslogWriter, "", 0)
	stdoutLogger := log.New(os.Stdout, "", 0)
	stderrLogger := log.New(os.Stderr, "", 0)

	loggers = []*log.Logger{syslogLogger, stdoutLogger, stderrLogger}
	return nil
}
