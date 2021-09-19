package logger

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/viper"
)

var logger *log.Logger

func init() {
	logger = log.New(ioutil.Discard, "[github-apps-trampoline] ", 0)
	Refresh()
}

func Refresh() {
	if viper.GetBool("verbose") {
		logger.SetOutput(os.Stderr)
	}
}

func Get() *log.Logger {
	return logger
}
