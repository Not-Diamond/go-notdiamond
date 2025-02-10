package notdiamond

import (
	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

func init() {
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// SetLevel sets the logging level
func setLevel(level string) {
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warn":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
}

func debugLog(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

func infoLog(args ...interface{}) {
	Log.Info(args...)
}

func warnLog(args ...interface{}) {
	Log.Warn(args...)
}

func errorLog(args ...interface{}) {
	Log.Error(args...)
}
