package logproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	deliveryId = "46a4c277-fa34-4711-b2eb-f6903bb06ce5"
	timestamp  = 1661772694
	cluster    = "test"
	project    = "test-dev-1234"
)

func TestHandleFunc(t *testing.T) {
	gcpEnabled := Config{
		Projects: map[string]string{
			cluster: project,
		},
	}
	tests := []struct {
		name     string
		url      string
		code     int
		location string
		cfg      Config
	}{
		{
			name:     "happy path kibana",
			url:      fmt.Sprintf("/logs?delivery_id=%s&ts=%d&v=1", deliveryId, timestamp),
			code:     http.StatusTemporaryRedirect,
			location: "https://logs.adeo.no/app/kibana#/discover?_a=(index:'96e648c0-980a-11e9-830a-e17bbd64b4db',query:(language:lucene,query:'+x_correlation_id:\"46a4c277-fa34-4711-b2eb-f6903bb06ce5\" -level:\"Trace\" -level:\"Debug\"'))&_g=(time:(from:'2022-08-29T00:00:00Z',mode:absolute,to:'2022-08-30T00:00:00Z'))",
		},
		{
			name:     "happy path gcp",
			url:      fmt.Sprintf("/logs?delivery_id=%s&ts=%d&v=1&cluster=%s", deliveryId, timestamp, cluster),
			code:     http.StatusTemporaryRedirect,
			location: "https://console.cloud.google.com/logs/query;query=jsonPayload.correlation_id%3d%2246a4c277-fa34-4711-b2eb-f6903bb06ce5%22;timeRange=2022-08-29T09:31:34Z%2f2022-08-29T13:31:34Z?authuser=0&project=test-dev-1234",
			cfg:      gcpEnabled,
		},
		{
			name: "bad delivery ID",
			url:  fmt.Sprintf("/logs?delivery_id=%s&ts=%d&v=1", "bad-uuid", timestamp),
			code: http.StatusBadRequest,
		},
		{
			name: "bad timestamp",
			url:  fmt.Sprintf("/logs?delivery_id=%s&ts=%s&v=1", deliveryId, "bad-ts"),
			code: http.StatusBadRequest,
		},
		{
			name:     "missing version",
			url:      fmt.Sprintf("/logs?delivery_id=%s&ts=%d", deliveryId, timestamp),
			code:     http.StatusTemporaryRedirect,
			location: "https://logs.adeo.no/app/kibana#/discover?_a=(index:'96e648c0-980a-11e9-830a-e17bbd64b4db',query:(language:lucene,query:'+x_delivery_id:\"46a4c277-fa34-4711-b2eb-f6903bb06ce5\" -level:\"Trace\"'))&_g=(time:(from:'2022-08-29T00:00:00Z',mode:absolute,to:'2022-08-30T00:00:00Z'))",
		},
		{
			name: "bad version",
			url:  fmt.Sprintf("/logs?delivery_id=%s&ts=%d&v=100", "bad-uuid", timestamp),
			code: http.StatusBadRequest,
		},
		{
			name: "bad cluster",
			url:  fmt.Sprintf("/logs?delivery_id=%s&ts=%d&v=1&cluster=bad-cluster", deliveryId, timestamp),
			code: http.StatusBadRequest,
			cfg:  gcpEnabled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			rr := httptest.NewRecorder()
			MakeHandler(tt.cfg)(rr, req)
			assert.Equal(t, tt.code, rr.Code)
			assert.Equal(t, tt.location, rr.Header().Get("Location"))
		})
	}
}
