package pb

import (
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
)

func TimestampAsTime(timestamp *timestamp.Timestamp) time.Time {
	return time.Unix(timestamp.GetSeconds(), int64(timestamp.GetNanos()))
}

func TimeAsTimestamp(t time.Time) *timestamp.Timestamp {
	return &timestamp.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

func (x *DeploymentRequest) Timestamp() time.Time {
	return TimestampAsTime(x.GetTime())
}

func (x *DeploymentStatus) Timestamp() time.Time {
	return TimestampAsTime(x.GetTime())
}
