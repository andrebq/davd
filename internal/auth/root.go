package auth

import (
	"context"

	"github.com/andrebq/davd/internal/config"
)

type (
	User struct {
		Name        string `msgpack:"name"`
		SaltedToken []byte `msgpack:"salted_token"`
	}

	ErrInvalidCredentials struct{}
)

func (ErrInvalidCredentials) Error() string {
	return "auth: invalid credentials"
}

func InitRoot(ctx context.Context, db *config.DB, token string) (bool, error) {
	var root User
	key := config.Key("users/root/profile")
	err := db.Get(ctx, &root, key)
	if config.NotFound(err) {
		root.Name = "root"
		root.SaltedToken, err = saltPassword(token, nil)
		if err != nil {
			return false, err
		}
		err = db.Put(ctx, key, root)
		return err == nil, err
	}
	return false, nil
}
