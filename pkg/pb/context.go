package pb

import (
	"context"
	"time"
)

func (m *DeploymentRequest) Context() (context.Context, context.CancelFunc) {
	deadline := time.Unix(m.GetDeadline(), 0)
	return context.WithDeadline(context.Background(), deadline)
}
