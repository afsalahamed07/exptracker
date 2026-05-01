package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Level int

const (
	debugLevel Level = iota
	infoLevel
	warnLevel
	errorLevel
)

type Logger struct {
	level Level
	base  *log.Logger
}

func New(value string) Logger {
	return Logger{
		level: parseLevel(value),
		base:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func parseLevel(value string) Level {
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

func (l Logger) Debugf(format string, args ...any) {
	l.logf(debugLevel, "DEBUG", format, args...)
}

func (l Logger) Infof(format string, args ...any) {
	l.logf(infoLevel, "INFO", format, args...)
}

func (l Logger) Warnf(format string, args ...any) {
	l.logf(warnLevel, "WARN", format, args...)
}

func (l Logger) Errorf(format string, args ...any) {
	l.logf(errorLevel, "ERROR", format, args...)
}

func (l Logger) logf(level Level, label, format string, args ...any) {
	if level < l.level {
		return
	}
	base := l.base
	if base == nil {
		base = log.New(os.Stdout, "", log.LstdFlags)
	}
	base.Printf("[%s] %s", label, fmt.Sprintf(format, args...))
}
