package api_v1

import (
	"context"
	"fmt"
)

func GroupClaims(ctx context.Context) ([]string, error) {
	groups, ok := ctx.Value("groups").([]string)
	if !ok {
		return nil, fmt.Errorf("no group claims found in context")
	}
	return groups, nil
}
