package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"log/slog"

	"github.com/andrebq/davd/internal/config"
	"golang.org/x/crypto/argon2"
)

func CheckCredential(ctx context.Context, db *config.DB, user string, plainTextPwd string) error {
	key := config.Key("users", user, "profile")
	var profile User
	err := db.Get(ctx, &profile, key)
	if err != nil {
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
	if subtle.ConstantTimeCompare(salted, profile.SaltedToken) != 0 {
		return ErrInvalidCredentials{}
	}
	return nil
}

func saltPassword(t string, salt []byte) ([]byte, error) {
	if salt == nil {
		salt := make([]byte, 8)
		_, err := rand.Read(salt)
		if err != nil {
			return nil, err
		}
	}
	val := argon2.IDKey([]byte(t), salt, 2, 32*1024, 4, 32)
	return append(val, salt[:]...), nil
}
