//go:build linux

package logger

import (
	"io"
	"log"
	"log/syslog"
)

func initPlatformLogger(tag string, extraLoggers []io.Writer) error {
	syslogWriter, err := syslog.New(syslog.LOG_INFO, tag)
	if err != nil {
		return err
	}
	syslogLogger := log.New(syslogWriter, "", 0)
	loggers = append(loggers, syslogLogger)

	for _, l := range extraLoggers {
		loggers = append(loggers, log.New(l, "", 0))
	}

	return nil
}
