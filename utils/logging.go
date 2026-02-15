package utils

import "github.com/sirupsen/logrus"

var (
	Logger = logrus.New()
	logger = Logger
)

func SetVerbose() {
	Logger.SetLevel(logrus.DebugLevel)
}
