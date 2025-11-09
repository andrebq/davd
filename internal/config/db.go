package config

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

type (
	Bindings map[string]string

	DB struct {
		abs                   string
		tokenSignKey          [32]byte
		tokenEncryptionKey    [32]byte
		passwordEncryptionKey [32]byte
		storageEncryptionKey  [32]byte
		tokenSettings         struct {
			Issuer string
		}
	}

	initialSetup struct {
		Done bool `json:"done"`
	}

	User struct {
		Name        string       `json:"name"`
		Email       string       `json:"email,omitempty"`
		Admin       bool         `json:"admin"`
		Active      bool         `json:"active"`
		Permissions []Permission `json:"permissions,omitempty"`
	}

	Permission struct {
		Prefix  string `json:"prefix"`
		Reader  bool   `json:"reader"`
		Writer  bool   `json:"writer"`
		Execute bool   `json:"execute"`
	}
)

const (
	ConfigBind = "_sysconfig"
)

var (
	ErrNoConfigBinding = errors.New("no configuration binding provided")
)

func Open(ctx context.Context,
	configdir string,
	env func(string) string) (*DB, error) {
	localpath, err := filepath.Abs(configdir)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(localpath, 0755)
	if err != nil {
		return nil, err
	}

	seed, err := hex.DecodeString(os.Getenv("DAVD_SEED_KEY"))
	if err != nil {
		return nil, err
	} else if len(seed) != 32 {
		return nil, errors.New("invalid DAVD_SEED_KEY: must be 64 hex characters (32 bytes)")
	}

	db := &DB{
		abs:                  localpath,
		tokenSignKey:         deriveKey(seed, []byte("token-signing"), []byte{01}),
		tokenEncryptionKey:   deriveKey(seed, []byte("token-encryption"), []byte{01}),
		storageEncryptionKey: deriveKey(seed, []byte("storage-encryption"), []byte{01}),
	}
	db.tokenSettings.Issuer = env("DAVD_SELF_TOKEN_ISSUER")
	if db.tokenSettings.Issuer == "" {
		db.tokenSettings.Issuer = "davd-server"
	}
	return db, nil
}

func (db *DB) InitialSetup() (bool, error) {
	var is initialSetup
	err := db.loadJSON(&is, "initial_setup")
	if errors.Is(err, ErrNoSuchKey) {
		err = nil
	} else if err != nil {
		return false, err
	}
	if is.Done {
		slog.Warn("Initial setup already completed. Ignoring admin token")
		return false, nil
	}
	if err := db.CreateUser("admin", true); err != nil {
		return false, err
	}
	// if err := db.RegisterToken("admin", adminToken); err != nil {
	// 	return err
	// }
	is.Done = true
	db.storeJSON(&is, "initial_setup")
	return is.Done, db.storeJSON("initial_setup")
}

func deriveKey(seed, info, secret_salt []byte) [32]byte {
	h := hmac.New(sha256.New, seed)
	h.Write(info)
	h.Write(secret_salt)
	var sum [32]byte
	h.Sum(sum[:0])
	return sum
}
