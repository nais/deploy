package pb

import (
	"context"
)

func (m *DeploymentRequest) Context() (context.Context, context.CancelFunc) {
	deadline := TimestampAsTime(m.Deadline)
	return context.WithDeadline(context.Background(), deadline)
}
