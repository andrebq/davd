package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/andrebq/davd/internal/config"
	"golang.org/x/crypto/argon2"
)

type (
	Permissions struct {
		CanWrite bool
		Allowed  []string
	}

	ctxkey byte
)

const (
	permissionsCtxKey = ctxkey(1)
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

func WithPermissions(ctx context.Context, perm Permissions) context.Context {
	return context.WithValue(ctx, permissionsCtxKey, perm)
}

func GetPermissions(ctx context.Context) Permissions {
	val := ctx.Value(permissionsCtxKey)
	if val == nil {
		return Permissions{CanWrite: false}
	}
	return val.(Permissions)
}

func UpdatePermissions(ctx context.Context, db *config.DB, user string, permissions Permissions) error {
	key := userProfileKey(User{Name: user})
	var profile User
	err := db.Get(ctx, &profile, key)
	if err != nil {
		return fmt.Errorf("unable to get desired user profile: %w", err)
	}
	cleanPath := func(v string) string {
		return fmt.Sprintf("/%v/", strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(v), "/"), "/"))
	}
	for i, v := range permissions.Allowed {
		permissions.Allowed[i] = cleanPath(v)
	}
	if profile.Permissions != nil {
		for _, v := range profile.Permissions.Allowed {
			permissions.Allowed = append(permissions.Allowed, cleanPath(v))
		}
	}
	sort.Strings(permissions.Allowed)
	dedup := make([]string, 0, len(permissions.Allowed))
	for i, v := range permissions.Allowed {
		if i == 0 {
			dedup = append(dedup, v)
			continue
		}
		if strings.HasPrefix(v, permissions.Allowed[i-1]) {
			// v is a sub-path of the previous path
			// therefore we don't need to check V
			// since the parent permission already handles it
			continue
		}
		dedup = append(dedup, v)
	}
	permissions.Allowed = dedup
	profile.Permissions = &permissions
	return db.Put(ctx, key, profile)
}

func CheckCredential(ctx context.Context, db *config.DB, user string, plainTextPwd string) (*Permissions, error) {
	key := userProfileKey(User{Name: user})
	var profile User
	err := db.Get(ctx, &profile, key)
	if err != nil {
		if config.NotFound(err) {
			return nil, ErrInvalidCredentials{}
		}
		return nil, err
	}
	if len(profile.SaltedToken) != 40 {
		slog.Error("auth: user profile without proper length")
		return nil, ErrInvalidCredentials{}
	}
	salt := profile.SaltedToken[32:]
	salted, err := saltPassword(plainTextPwd, salt)
	if err != nil {
		return nil, ErrInvalidCredentials{}
	}
	if subtle.ConstantTimeCompare(salted, profile.SaltedToken) != 1 {
		return nil, ErrInvalidCredentials{}
	}
	if profile.Permissions == nil {
		return &Permissions{}, nil
	}
	return profile.Permissions, nil
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
