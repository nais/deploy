package middleware

import "context"

const contextKeyEmail = "email"
const contextKeyGroups = "groups"

func GetEmail(ctx context.Context) string {
	email, _ := ctx.Value(contextKeyEmail).(string)
	return email
}

func GetGroups(ctx context.Context) []string {
	groups, _ := ctx.Value(contextKeyGroups).([]string)
	return groups
}

func WithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, contextKeyEmail, email)
}

func WithGroups(ctx context.Context, groups []string) context.Context {
	return context.WithValue(ctx, contextKeyGroups, groups)
}
