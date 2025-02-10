package notdiamond

import (
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func init() {
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// SetLevel sets the logging level
func setLevel(level string) {
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
}

func debugLog(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func infoLog(args ...interface{}) {
	logger.Info(args...)
}

func warnLog(args ...interface{}) {
	logger.Warn(args...)
}

func errorLog(args ...interface{}) {
	logger.Error(args...)
}
