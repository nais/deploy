package logproxy

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	deliveryId = "46a4c277-fa34-4711-b2eb-f6903bb06ce5"
	timestamp  = 1661772694
)

func TestHandleFunc(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		code     int
		location string
	}{
		{
			name:     "happy path",
			url:      fmt.Sprintf("/logs?delivery_id=%s&ts=%d&v=1", deliveryId, timestamp),
			code:     http.StatusTemporaryRedirect,
			location: "https://logs.adeo.no/app/kibana#/discover?_a=(index:'96e648c0-980a-11e9-830a-e17bbd64b4db',query:(language:lucene,query:'+x_correlation_id:\"46a4c277-fa34-4711-b2eb-f6903bb06ce5\" -level:\"Trace\" -level:\"Debug\"'))&_g=(time:(from:'2022-08-29T00:00:00Z',mode:absolute,to:'2022-08-30T00:00:00Z'))",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			rr := httptest.NewRecorder()
			HandleFunc(rr, req)
			assert.Equal(t, tt.code, rr.Code)
			assert.Equal(t, tt.location, rr.Header().Get("Location"))
		})
	}
}
