package server

import (
	"fmt"
	"sync"
)

type ServerLogger interface {
	Error(v ...interface{}) error
	Warning(v ...interface{}) error
	Info(v ...interface{}) error

	Errorf(format string, a ...interface{}) error
	Warningf(format string, a ...interface{}) error
	Infof(format string, a ...interface{}) error
}

var (
	logger      ServerLogger
	loggerMutex sync.Mutex
)

func init() {
	logger = &DefaultLogger{}
}

func SetLogger(l ServerLogger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	logger = l
}

type DefaultLogger struct{}

type Level int

const (
	Error   Level = 3
	Warning Level = 4
	Info    Level = 6
)

func (l Level) String() string {
	switch l {
	case Error:
		return "Error"
	case Warning:
		return "Warning"
	case Info:
		return "Info"
	}
	return "Unknown log level"
}

func (l DefaultLogger) log(level Level, v ...interface{}) {
	fmt.Printf("%s: %s", level, fmt.Sprintln(v...))
}

func (l DefaultLogger) logf(level Level, format string, v ...interface{}) {
	fmt.Printf("%s: %s\n", level, fmt.Sprintf(format, v...))
}

func (l DefaultLogger) Error(v ...interface{}) error {
	l.log(Error, v...)
	return nil
}
func (l DefaultLogger) Warning(v ...interface{}) error {
	l.log(Warning, v...)
	return nil
}
func (l DefaultLogger) Info(v ...interface{}) error {
	l.log(Info, v...)
	return nil
}

func (l DefaultLogger) Errorf(format string, a ...interface{}) error {
	l.logf(Error, format, a...)
	return nil
}

func (l DefaultLogger) Warningf(format string, a ...interface{}) error {
	l.logf(Warning, format, a...)
	return nil
}
func (l DefaultLogger) Infof(format string, a ...interface{}) error {
	l.logf(Info, format, a...)
	return nil
}
