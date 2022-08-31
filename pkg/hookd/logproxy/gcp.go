package logproxy

import (
	"fmt"
	"time"
)

const gcpFormat = "https://console.cloud.google.com/logs/query?query=jsonPayload.correlation_id=%%22%s%%22&timeRange=PT1D&authuser=0&project=%s&cursorTimestamp=%s"

type gcpFormatter struct {
	Projects map[string]string
}

func (f gcpFormatter) format(deliveryID string, ts time.Time, _ int, cluster string) (string, error) {
	if project, ok := f.Projects[cluster]; ok {
		return fmt.Sprintf(gcpFormat, deliveryID, project, ts.Format(time.RFC3339)), nil
	}
	return "", fmt.Errorf("unknown cluster %s", cluster)
}
