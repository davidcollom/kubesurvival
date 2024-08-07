package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// LogLevelHook is a custom hook for logrus to send different levels of logs to different outputs and formats
type LogLevelHook struct {
	Writer     map[logrus.Level]*os.File
	Formatters map[logrus.Level]logrus.Formatter
	LogLevels  []logrus.Level
}

// NewLogLevelHook initializes the custom hook
func NewLogLevelHook() *LogLevelHook {
	defaultLoggerFormat := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	return &LogLevelHook{
		Writer: map[logrus.Level]*os.File{
			logrus.DebugLevel: os.Stderr,
			logrus.InfoLevel:  os.Stdout,
			logrus.WarnLevel:  os.Stderr,
			logrus.ErrorLevel: os.Stderr,
			logrus.FatalLevel: os.Stderr,
			logrus.PanicLevel: os.Stderr,
		},
		Formatters: map[logrus.Level]logrus.Formatter{
			logrus.DebugLevel: defaultLoggerFormat,
			logrus.InfoLevel:  &SimpleFormatter{},
			logrus.WarnLevel:  defaultLoggerFormat,
			logrus.ErrorLevel: defaultLoggerFormat,
			logrus.FatalLevel: defaultLoggerFormat,
			logrus.PanicLevel: defaultLoggerFormat,
		},
		LogLevels: logrus.AllLevels,
	}
}

// Levels defines on which log levels this hook would trigger
func (hook *LogLevelHook) Levels() []logrus.Level {
	return hook.LogLevels
}

// Fire is called by logrus when a log entry needs to be logged
func (hook *LogLevelHook) Fire(entry *logrus.Entry) error {
	writer, ok := hook.Writer[entry.Level]
	if !ok {
		writer = os.Stdout
	}

	formatter, ok := hook.Formatters[entry.Level]
	if !ok {
		formatter = hook.Formatters[logrus.InfoLevel]
	}

	// Format the entry using the appropriate formatter
	bytes, err := formatter.Format(entry)
	if err != nil {
		return err
	}

	_, err = writer.Write(bytes)
	return err
}

// SimpleFormatter is a custom formatter that only outputs the message
type SimpleFormatter struct{}

func (f *SimpleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message + "\n"), nil
}
