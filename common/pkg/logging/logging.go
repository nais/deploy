package logging

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

func textFormatter() log.Formatter {
	return &log.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
	}
}

func jsonFormatter() log.Formatter {
	return &log.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	}
}

func Setup(level, format string) error {
	switch format {
	case "json":
		log.SetFormatter(jsonFormatter())
	case "text":
		log.SetFormatter(textFormatter())
	default:
		return fmt.Errorf("log format '%s' is not recognized", format)
	}

	logLevel, err := log.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("while setting log level: %s", err)
	}
	log.SetLevel(logLevel)

	return nil
}

func New(level, format string) (*log.Logger, error) {
	logger := log.New()

	switch format {
	case "json":
		logger.SetFormatter(jsonFormatter())
	case "text":
		logger.SetFormatter(textFormatter())
	default:
		return nil, fmt.Errorf("log format '%s' is not recognized", format)
	}

	logLevel, err := log.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("while setting log level: %s", err)
	}
	logger.SetLevel(logLevel)

	return logger, nil
}
