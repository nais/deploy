package telemetry_test

import (
	"context"
	"testing"

	"github.com/nais/deploy/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestTraceID(t *testing.T) {
	t.Run("happy case", func(t *testing.T) {
		traceParentHeader := "00-ada6313c1a5b6ffdf0d085fadc3265cb-6018288557ffff51-01"
		ctx := telemetry.WithTraceParent(context.Background(), traceParentHeader)
		traceID := telemetry.TraceID(ctx)
		assert.Equal(t, "ada6313c1a5b6ffdf0d085fadc3265cb", traceID)
	})

	t.Run("invalid data", func(t *testing.T) {
		traceParentHeader := "some-invalid-data"
		ctx := telemetry.WithTraceParent(context.Background(), traceParentHeader)
		traceID := telemetry.TraceID(ctx)
		assert.Equal(t, "", traceID)
	})

	t.Run("no data", func(t *testing.T) {
		traceParentHeader := ""
		ctx := telemetry.WithTraceParent(context.Background(), traceParentHeader)
		traceID := telemetry.TraceID(ctx)
		assert.Equal(t, "", traceID)
	})
}
