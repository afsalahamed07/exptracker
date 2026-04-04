package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type logLevel int

const (
	debugLevel logLevel = iota
	infoLevel
	warnLevel
	errorLevel
)

type appLogger struct {
	level logLevel
	base  *log.Logger
}

func newLogger(value string) appLogger {
	return appLogger{
		level: parseLogLevel(value),
		base:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func parseLogLevel(value string) logLevel {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return debugLevel
	case "warn":
		return warnLevel
	case "error":
		return errorLevel
	default:
		return infoLevel
	}
}

func (l appLogger) Debugf(format string, args ...interface{}) {
	l.logf(debugLevel, "DEBUG", format, args...)
}

func (l appLogger) Infof(format string, args ...interface{}) {
	l.logf(infoLevel, "INFO", format, args...)
}

func (l appLogger) Warnf(format string, args ...interface{}) {
	l.logf(warnLevel, "WARN", format, args...)
}

func (l appLogger) Errorf(format string, args ...interface{}) {
	l.logf(errorLevel, "ERROR", format, args...)
}

func (l appLogger) logf(level logLevel, label, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	base := l.base
	if base == nil {
		base = log.New(os.Stdout, "", log.LstdFlags)
	}
	base.Printf("[%s] %s", label, fmt.Sprintf(format, args...))
}
