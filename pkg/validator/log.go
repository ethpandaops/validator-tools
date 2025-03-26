package validator

import (
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	log = logrus.New()

	log.SetLevel(logrus.InfoLevel)
}

// GetLogger returns the configured logger instance
func GetLogger() *logrus.Logger {
	return log
}

// SetLogLevel sets the logging level
func SetLogLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}

	log.SetLevel(lvl)

	return nil
}
