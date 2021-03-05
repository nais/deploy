package deployer

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

func SetupLogging(cfg Config) {
	log.SetOutput(os.Stderr)

	if cfg.Actions {
		log.SetFormatter(&ActionsFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp:          true,
			TimestampFormat:        time.RFC3339Nano,
			DisableLevelTruncation: true,
		})
	}

	if cfg.Quiet {
		log.SetLevel(log.ErrorLevel)
	}
}
