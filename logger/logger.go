package logger

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/viper"
)

var (
	combinedLogger *log.Logger
	fileLogger     *log.Logger
	stderrLogger   *log.Logger
	logFilePath    string
	logFile        *os.File
)

func init() {
	combinedLogger = log.New(io.Discard, "[github-apps-trampoline] ", 0)
	fileLogger = log.New(io.Discard, "[github-apps-trampoline] ", 0)
	stderrLogger = log.New(io.Discard, "[github-apps-trampoline] ", 0)
	Refresh()
}

func Refresh() {
	fileWriter := io.Discard
	stderrWriter := io.Discard

	configuredPath := viper.GetString("log-file")
	if configuredPath != "" {
		if logFile == nil || configuredPath != logFilePath {
			if logFile != nil {
				_ = logFile.Close()
				logFile = nil
			}
			f, err := os.OpenFile(configuredPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				fmt.Fprintf(os.Stderr, "github-apps-trampoline: failed to open log file %q: %v\n", configuredPath, err)
			} else {
				logFile = f
				logFilePath = configuredPath
			}
		}
		if logFile != nil {
			fileWriter = logFile
		}
	}

	if viper.GetBool("verbose") || viper.GetBool("log-tee-stderr") {
		stderrWriter = os.Stderr
	}

	combinedWriters := []io.Writer{}
	if fileWriter != io.Discard {
		combinedWriters = append(combinedWriters, fileWriter)
	}
	if stderrWriter != io.Discard {
		combinedWriters = append(combinedWriters, stderrWriter)
	}

	if len(combinedWriters) == 0 {
		combinedLogger.SetOutput(io.Discard)
	} else {
		combinedLogger.SetOutput(io.MultiWriter(combinedWriters...))
	}

	fileLogger.SetOutput(fileWriter)
	stderrLogger.SetOutput(stderrWriter)
}

func Get() *log.Logger {
	return combinedLogger
}

func File() *log.Logger {
	return fileLogger
}

func StderrRedacted() *log.Logger {
	return stderrLogger
}

func Sensitivef(format string, args ...interface{}) {
	File().Printf(format, args...)
	StderrRedacted().Printf(format, args...)
}

func Filef(format string, args ...interface{}) {
	File().Printf(format, args...)
}

func Stderrf(format string, args ...interface{}) {
	StderrRedacted().Printf(format, args...)
}
