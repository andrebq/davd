package config

import "context"

func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey{}).(*User)
	return user
}

type userContextKey struct{}
