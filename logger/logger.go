package logger

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/viper"
)

var (
	logger      *log.Logger
	logFilePath string
	logFile     *os.File
)

func init() {
	logger = log.New(io.Discard, "[github-apps-trampoline] ", 0)
	Refresh()
}

func Refresh() {
	writers := []io.Writer{}

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
			writers = append(writers, logFile)
		}
	}

	if viper.GetBool("verbose") || viper.GetBool("log-tee-stderr") {
		writers = append(writers, os.Stderr)
	}

	if len(writers) == 0 {
		logger.SetOutput(io.Discard)
		return
	}
	logger.SetOutput(io.MultiWriter(writers...))
}

func Get() *log.Logger {
	return logger
}
