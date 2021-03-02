package pb

import (
	"context"
)

func (x *DeploymentRequest) Context() (context.Context, context.CancelFunc) {
	deadline := TimestampAsTime(x.Deadline)
	return context.WithDeadline(context.Background(), deadline)
}
