package config

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

type (
	TokenInfo struct {
		raw     string
		decoded bool
		claims  jwt.RegisteredClaims
	}
)

func (db *DB) decodeToken(info *TokenInfo) error {
	return errors.ErrUnsupported
}
