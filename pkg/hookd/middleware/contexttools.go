package middleware

import (
	"context"
	"fmt"
)

const (
	contextKeyEmail  = "email"
	contextKeyGroups = "groups"
)

func GetEmail(ctx context.Context) string {
	email, _ := ctx.Value(contextKeyEmail).(string)
	return email
}

func GetGroups(ctx context.Context) ([]string, error) {
	groups, ok := ctx.Value(contextKeyGroups).([]string)
	if !ok {
		return nil, fmt.Errorf("no group claims found in context")
	}
	return groups, nil
}

func WithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, contextKeyEmail, email)
}

func WithGroups(ctx context.Context, groups []string) context.Context {
	return context.WithValue(ctx, contextKeyGroups, groups)
}
