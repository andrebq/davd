package auth

import (
	"context"

	"github.com/andrebq/davd/internal/config"
)

type (
	User struct {
		Name        string `msgpack:"name"`
		SaltedToken []byte `msgpack:"salted_token"`

		Permissions *Permissions `msgpack:"permissions"`
	}

	ErrInvalidCredentials struct{}
)

func (ErrInvalidCredentials) Error() string {
	return "auth: invalid credentials"
}

func InitRoot(ctx context.Context, db *config.DB, token string) (bool, error) {
	var root User
	root.Name = "root"
	key := userProfileKey(root)
	err := db.Get(ctx, &root, key)
	if config.NotFound(err) {
		err = putUser(ctx, db, root, token)
		return err == nil, err
	}
	return false, nil
}
