// Copyright 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
package logger

import (
	"fmt"
	"log"
	"os"
)

const (
	capriceLogPrefix = "[caprice] "
)

type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Panic(format string, v ...interface{})
	Fault(format string, v ...interface{})
}

var (
	stdLogger      = NewDefaultLogger(0)
	globalLogger   = Logger(stdLogger)
	disableLogging bool
)

func SetLogger(l Logger) {
	globalLogger = l
}

func EnableLogging() {
	disableLogging = false
}

func DisableLogging() {
	disableLogging = true
}

func Debug(format string, v ...interface{}) {
	if globalLogger != nil && !disableLogging {
		globalLogger.Debug(capriceLogPrefix+format, v...)
	}
}

func Info(format string, v ...interface{}) {
	if globalLogger != nil && !disableLogging {
		globalLogger.Info(capriceLogPrefix+format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if globalLogger != nil && !disableLogging {
		globalLogger.Warn(capriceLogPrefix+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if globalLogger != nil && !disableLogging {
		globalLogger.Error(capriceLogPrefix+format, v...)
	}
}

func Panic(format string, v ...interface{}) {
	if globalLogger != nil && !disableLogging {
		globalLogger.Panic(capriceLogPrefix+format, v...)
	}
}

func Fault(format string, v ...interface{}) {
	if globalLogger != nil && !disableLogging {
		globalLogger.Fault(capriceLogPrefix+format, v...)
	}
}

// DefaultLogger is a default implementation of the Logger interface.
type DefaultLogger struct {
	*log.Logger
}

func NewDefaultLogger(level int) *DefaultLogger {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	return &DefaultLogger{
		Logger: logger,
	}
}

func (l *DefaultLogger) header(lvl, msg string) string {
	return fmt.Sprintf("%s: %s", lvl, msg)
}

func (l *DefaultLogger) Debug(format string, v ...interface{}) {
	_ = l.Output(4, l.header("[DEBUG]", fmt.Sprintf(format, v...)))
}

func (l *DefaultLogger) Info(format string, v ...interface{}) {
	_ = l.Output(4, l.header("[INFO.]", fmt.Sprintf(format, v...)))
}

func (l *DefaultLogger) Warn(format string, v ...interface{}) {
	_ = l.Output(4, l.header("[WARN.]", fmt.Sprintf(format, v...)))
}

func (l *DefaultLogger) Error(format string, v ...interface{}) {
	_ = l.Output(4, l.header("[ERROR]", fmt.Sprintf(format, v...)))
}

func (l *DefaultLogger) Panic(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	_ = l.Output(4, l.header("[PANIC]", msg))
	panic(msg)
}

func (l *DefaultLogger) Fault(format string, v ...interface{}) {
	_ = l.Output(4, l.header("[FAULT]]", fmt.Sprintf(format, v...)))
	os.Exit(-1)
}
