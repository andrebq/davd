package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"log/slog"

	"github.com/andrebq/davd/internal/config"
	"golang.org/x/crypto/argon2"
)

func userProfileKey(user User) config.K {
	return config.Key("users", user.Name, "profile")
}

func UpsertUser(ctx context.Context, db *config.DB, user, passwd string) error {
	if user == "root" {
		return errors.New("root user cannot be modified")
	}
	return putUser(ctx, db, User{Name: user}, passwd)
}

func putUser(ctx context.Context, db *config.DB, user User, token string) error {
	key := userProfileKey(user)
	user.Name = "user"
	var err error
	user.SaltedToken, err = saltPassword(token, nil)
	if err != nil {
		return err
	}
	return db.Put(ctx, key, user)
}

func CheckCredential(ctx context.Context, db *config.DB, user string, plainTextPwd string) error {
	key := config.Key("users", user, "profile")
	var profile User
	err := db.Get(ctx, &profile, key)
	if err != nil {
		if config.NotFound(err) {
			return ErrInvalidCredentials{}
		}
		return err
	}
	if len(profile.SaltedToken) != 40 {
		slog.Error("auth: user profile without proper length")
		return ErrInvalidCredentials{}
	}
	salt := profile.SaltedToken[32:]
	salted, err := saltPassword(plainTextPwd, salt)
	if err != nil {
		return ErrInvalidCredentials{}
	}
	if subtle.ConstantTimeCompare(salted, profile.SaltedToken) != 1 {
		return ErrInvalidCredentials{}
	}
	return nil
}

func IsInvalidCredential(err error) bool {
	return errors.Is(err, ErrInvalidCredentials{})
}

func saltPassword(t string, salt []byte) ([]byte, error) {
	if salt == nil {
		salt = make([]byte, 8)
		_, err := rand.Read(salt)
		if err != nil {
			return nil, err
		}
	}
	val := argon2.IDKey([]byte(t), salt, 2, 32*1024, 4, 32)
	return append(val, salt[:]...), nil
}
