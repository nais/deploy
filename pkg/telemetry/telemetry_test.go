package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/nais/deploy/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestParsePipelineTelemetry(t *testing.T) {
	t.Run("default case with five timings in correct order without quoting", func(t *testing.T) {
		input := "latest_commit=1726040395,pipeline_start=1726050395,pipeline_end=1726050512,build_start=1726050400,attest_start=1726050492"
		expected := &telemetry.PipelineTimings{
			LatestCommit: time.Date(2024, time.September, 11, 7, 39, 55, 0, time.UTC),
			Start:        time.Date(2024, time.September, 11, 10, 26, 35, 0, time.UTC),
			BuildStart:   time.Date(2024, time.September, 11, 10, 26, 40, 0, time.UTC),
			AttestStart:  time.Date(2024, time.September, 11, 10, 28, 12, 0, time.UTC),
			End:          time.Date(2024, time.September, 11, 10, 28, 32, 0, time.UTC),
		}
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.NoError(t, err)
		assert.Equal(t, expected, output)
	})

	t.Run("missing some of the timings", func(t *testing.T) {
		input := "pipeline_start=1726050395,pipeline_end=1726050512"
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.EqualError(t, err, "pipeline timings are not in expected chronological order, ensure that: latest_commit < pipeline_start < build_start < attest_start < pipeline_end")
		assert.Nil(t, output)
	})

	t.Run("wrong timing order", func(t *testing.T) {
		for _, input := range []string{
			"pipeline_start=2,build_start=1",
			"build_start=2,attest_start=1",
			"attest_start=2,pipeline_end=1",
			"pipeline_start=2,pipeline_end=1",
		} {
			output, err := telemetry.ParsePipelineTelemetry(input)
			assert.EqualError(t, err, "pipeline timings are not in expected chronological order, ensure that: latest_commit < pipeline_start < build_start < attest_start < pipeline_end")
			assert.Nil(t, output)
		}
	})

	t.Run("unexpected timing parameter", func(t *testing.T) {
		input := "pipeline_start=1,foobar=2"
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.EqualError(t, err, "expected key to be one of 'latest_commit', 'pipeline_start', 'pipeline_end', 'build_start', 'attest_start'; found 'foobar'")
		assert.Nil(t, output)
	})

	t.Run("timing parameter not an integer", func(t *testing.T) {
		input := "pipeline_start=2024-09-11"
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.EqualError(t, err, "expected UNIX epoch, found '2024-09-11'")
		assert.Nil(t, output)
	})

	t.Run("parameter list missing value", func(t *testing.T) {
		input := "pipeline_start=1,pipeline_end"
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.EqualError(t, err, "expected 'key=value', found 'pipeline_end'")
		assert.Nil(t, output)
	})

	t.Run("parameter list missing key", func(t *testing.T) {
		input := "pipeline_start=1,=2"
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.EqualError(t, err, "expected key to be one of 'latest_commit', 'pipeline_start', 'pipeline_end', 'build_start', 'attest_start'; found ''")
		assert.Nil(t, output)
	})

	t.Run("no data", func(t *testing.T) {
		input := ""
		output, err := telemetry.ParsePipelineTelemetry(input)
		assert.NoError(t, err)
		assert.Nil(t, output)
	})
}

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
