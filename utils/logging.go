package utils

import "github.com/sirupsen/logrus"

var Logger = logrus.New()

func SetVerbose() {
	Logger.SetLevel(logrus.DebugLevel)
}
