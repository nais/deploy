package logproxy

import (
	"fmt"
	"time"
)

const (
	gcpFormat       = "https://console.cloud.google.com/logs/query;query=jsonPayload.correlation_id%%3d%%22%s%%22;timeRange=%s%%2f%s?authuser=0&project=%s"
	timeRangeMargin = 2 * time.Hour
)

type gcpFormatter struct {
	Projects map[string]string
}

func (f gcpFormatter) format(deliveryID string, ts time.Time, _ int, cluster string) (string, error) {
	if project, ok := f.Projects[cluster]; ok {
		start := ts.Add(-timeRangeMargin)
		end := ts.Add(timeRangeMargin)
		return fmt.Sprintf(gcpFormat, deliveryID, start.Format(time.RFC3339), end.Format(time.RFC3339), project), nil
	}
	return "", fmt.Errorf("unknown cluster %s", cluster)
}
