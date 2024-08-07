package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {

	// Set the output to Standard Err
	log.SetOutput(io.Discard)

	// Set the log level
	log.SetLevel(logrus.ErrorLevel)
	// log.SetReportCaller(true)

	// Register the custom hook
	log.AddHook(NewLogLevelHook())
}

// SetLogLevel sets the log level based on the provided string.
func SetLogLevel(level string) error {
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}
	return nil
}

// Logger returns the instance of the embedded logrus.Logger.
func Logger() *logrus.Logger {
	return log
}

// Info logs a message at level Info.
func Info(args ...interface{}) {
	log.Info(args...)
}

// Infof logs a formatted message at level Info.
func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Warn logs a message at level Warn.
func Warn(args ...interface{}) {
	log.Warn(args...)
}

// Warnf logs a formatted message at level Warn.
func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

// Error logs a message at level Error.
func Error(args ...interface{}) {
	log.Error(args...)
}

// Errorf logs a formatted message at level Error.
func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

// Debug logs a message at level Debug.
func Debug(args ...interface{}) {
	log.Debug(args...)
}

// Debugf logs a formatted message at level Debug.
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// Fatal logs a message at level Debug.
func Fatal(args ...interface{}) {
	log.Error(args...)
	os.Exit(1)
}

// Fatalf logs a formatted message at level Debug.
func Fatalf(format string, args ...interface{}) {
	log.Errorf(format, args...)
	os.Exit(1)
}
