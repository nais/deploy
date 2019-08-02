package logging

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

type standardLogger struct {
	logger *log.Logger
	level  log.Level
}

func (d *standardLogger) Print(v ...interface{}) {
	d.logger.Log(d.level, v...)
}

func (d *standardLogger) Printf(format string, v ...interface{}) {
	d.logger.Logf(d.level, format, v...)
}

func (d *standardLogger) Println(v ...interface{}) {
	d.logger.Logln(d.level, v...)
}

func New(level, format string) (*standardLogger, error) {
	var err error

	l := &standardLogger{}
	l.logger = log.New()

	switch format {
	case "json":
		l.logger.SetFormatter(JsonFormatter())
	case "text":
		l.logger.SetFormatter(TextFormatter())
	default:
		return nil, fmt.Errorf("log format '%s' is not recognized", format)
	}

	l.level, err = log.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("while setting log level: %s", err)
	}
	l.logger.SetLevel(log.GetLevel())

	return l, nil
}
