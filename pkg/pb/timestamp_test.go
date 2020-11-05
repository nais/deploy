package pb_test

import (
	"testing"
	"time"

	"github.com/navikt/deployment/pkg/pb"
	"github.com/stretchr/testify/assert"
)

func TestTimestampConversion(t *testing.T) {
	now := time.Now()

	ts := pb.TimeAsTimestamp(now)
	converted := pb.TimestampAsTime(ts)

	assert.Equal(t, now.Unix(), ts.GetSeconds())
	assert.EqualValues(t, now.Nanosecond(), ts.GetNanos())

	// Sub-nanosecond information is lost in time conversion.
	assert.Equal(t, now.Unix(), converted.Unix())
	assert.Equal(t, now.Nanosecond(), converted.Nanosecond())
	assert.Equal(t, now.UnixNano(), converted.UnixNano())
}
