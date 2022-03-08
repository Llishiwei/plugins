package log

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/sirupsen/logrus"
)

var (
	logFile  *os.File
	logger   *logrus.Logger
	logLevel logrus.Level
)

func Init(logFilePath, logFileName string, level logrus.Level) error {
	err := os.MkdirAll(logFilePath, 0755)
	if err != nil {
		return err
	}
	fileName := path.Join(logFilePath, logFileName)
	logFile, err = os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	logger = logrus.New()
	logger.SetOutput(logFile)
	logLevel = level
	logger.SetLevel(logLevel)
	logger.SetFormatter(&logrus.TextFormatter{})
	return nil
}

func Close() error {
	if logFile == nil {
		return nil
	}
	return logFile.Close()
}

func Debugf(format string, a ...interface{}) {
	if logLevel < logrus.DebugLevel {
		return
	}
	_, file, line, ok := runtime.Caller(1)

	fullMsg := fmt.Sprintf(format, a...)

	if ok {
		fullMsg = fmt.Sprintf("file: %v, line: %v. %s", file, line, fullMsg)
	}

	logger.Debug(fullMsg)
}

func Infof(format string, a ...interface{}) {
	if logLevel < logrus.InfoLevel {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	fullMsg := fmt.Sprintf(format, a...)

	if ok {
		fullMsg = fmt.Sprintf("file: %v, line: %v. %s", file, line, fullMsg)
	}

	logger.Info(fullMsg)
}

func Warningf(format string, a ...interface{}) {
	if logLevel < logrus.WarnLevel {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	fullMsg := fmt.Sprintf(format, a...)

	if ok {
		fullMsg = fmt.Sprintf("file: %v, line: %v. %s", file, line, fullMsg)
	}

	logger.Warning(fullMsg)
}

func Errorf(format string, a ...interface{}) {
	if logLevel < logrus.ErrorLevel {
		return
	}
	_, file, line, ok := runtime.Caller(1)
	fullMsg := fmt.Sprintf(format, a...)

	if ok {
		fullMsg = fmt.Sprintf("file: %v, line: %v. %s", file, line, fullMsg)
	}

	logger.Error(fullMsg)
}
