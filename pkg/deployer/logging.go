package deployer

import (
	"bytes"
	"os"
	"time"

	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
)

type ActionsFormatter struct{}

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

func (a *ActionsFormatter) Format(e *log.Entry) ([]byte, error) {
	buf := &bytes.Buffer{}
	switch e.Level {
	case log.ErrorLevel:
		buf.WriteString("::error::")
	case log.WarnLevel:
		buf.WriteString("::warn::")
	default:
		buf.WriteString("[")
		buf.WriteString(e.Time.Format(time.RFC3339Nano))
		buf.WriteString("] ")
	}
	buf.WriteString(e.Message)
	buf.WriteRune('\n')
	return buf.Bytes(), nil
}

func logDeployStatus(status *pb.DeploymentStatus) {
	fn := log.Infof
	switch status.GetState() {
	case pb.DeploymentState_failure, pb.DeploymentState_error:
		fn = log.Errorf
	}
	fn("Deployment %s: %s", status.GetState(), status.GetMessage())
}
