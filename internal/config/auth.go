package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func (db *DB) verifyToken(info *TokenInfo) error {
	token, err := jwt.ParseWithClaims(info.raw, &info.claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return db.tokenSignKey[:], nil
	})
	if err != nil {
		return err
	}
	if token.Valid {
		info.decoded = true
		return nil
	}
	return fmt.Errorf("invalid token")
}

func (db *DB) generateToken(subject string, id []byte, ttl time.Duration) TokenInfo {
	if len(id) == 0 {
		id := make([]byte, 16)
		_, err := rand.Read(id)
		if err != nil {
			panic("FATAL RUNTIME ERROR: unable to read from crypto/rand")
		}
	}
	claims := jwt.RegisteredClaims{
		Issuer:    db.tokenSettings.Issuer,
		Subject:   subject,
		ID:        hex.EncodeToString(id),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(db.tokenSignKey[:])
	if err != nil {
		panic("FATAL RUNTIME ERROR: unable to sign token")
	}
	info := TokenInfo{
		raw:     token,
		decoded: true,
		claims:  claims,
	}
	return info
}

// CreateFullAccessToken creates a token with the same set of permissions as the given username.
//
// If apiKey is not empty, it will be used to encrypt the token data in the server database.
//
// Then, when a user provides its userName, apiKey and the tokenID, the server will be able to
// decrypt the token data and verify its permissions.
//
// This is mostly useful to allow users to login to the server with a password rather than
// by using Bearer tokens. If apiKey is empty, a random one will be generated
func (db *DB) CreateAPIKey(username string, ttl time.Duration) ([]byte, TokenInfo, error) {
	const (
		idLength  = 16
		keyLength = 32

		apiKeyLength = keyLength + idLength
	)
	var apiKey [apiKeyLength]byte
	_, err := rand.Read(apiKey[:])
	if err != nil {
		return nil, TokenInfo{}, err
	}
	id := apiKey[:idLength]
	key := apiKey[idLength:]
	token := db.generateToken(fmt.Sprintf("api_key:%s", username), id, ttl)

	tokenSealKey := deriveKey(db.tokenEncryptionKey[:], []byte("api_key"), key)
	keydata := struct {
		Token    []byte `json:"token"`
		Username string `json:"username"`
	}{
		Token:    encryptBuffer(&tokenSealKey, []byte(token.raw)),
		Username: username,
	}
	err = db.storeJSON(keydata, "api_keys", hex.EncodeToString(id))
	return apiKey[:], token, err
}

func (d *DB) UpsertUser(username, password string) error {
	if found, _ := d.FindUser(username); found == nil {
		if err := d.CreateUser(username, false); err != nil {
			return nil
		}
	}
	err := d.UpdatePassword(username, password)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) UpdatePassword(username, password string) error {
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	passwordKey := deriveKey(db.passwordEncryptionKey[:], []byte("user-password"), []byte(password))
	passwordObj := struct {
		Salted []byte `json:"bcrypt_hash"`
		Token  []byte `json:"access_token,omitempty"`
	}{
		Salted: bcryptHash,
		Token:  []byte(encryptBuffer(&passwordKey, []byte(db.generateToken(fmt.Sprintf("password:%v", username), nil, 10*365*24*time.Hour).raw))),
	}
	return db.storeEncryptedJSON(&passwordObj, append([]byte("passwords:"), []byte(username)...), "passwords", username)
}

func (db *DB) PasswordLogin(username, password string) (TokenInfo, *User, error) {
	var tokenInfo TokenInfo
	passwordObj := struct {
		Salted []byte `json:"bcrypt_hash"`
		Token  []byte `json:"access_token,omitempty"`
	}{}
	err := db.loadEncryptedJSON(&passwordObj, append([]byte("passwords:"), []byte(username)...), "passwords", username)
	if err != nil {
		return tokenInfo, nil, err
	}
	err = bcrypt.CompareHashAndPassword(passwordObj.Salted, []byte(password))
	if err != nil {
		return tokenInfo, nil, err
	}
	passwordKey := deriveKey(db.passwordEncryptionKey[:], []byte("user-password"), []byte(password))
	decryptedTokenBytes, err := decryptBuffer(&passwordKey, passwordObj.Token)
	if err != nil {
		return tokenInfo, nil, err
	}
	tokenInfo.raw = string(decryptedTokenBytes)
	err = db.verifyToken(&tokenInfo)
	if err != nil {
		return tokenInfo, nil, err
	}
	user, err := db.FindUser(username)
	if err != nil {
		return tokenInfo, nil, err
	}
	return tokenInfo, user, err
}

func (db *DB) UpdatePermissions(username string, permissions []Permission) error {
	user, err := db.FindUser(username)
	if err != nil {
		return err
	}
	user.Permissions = append(user.Permissions, permissions...)
	// TODO: should perform a smarter deduplication in case a prefix already
	slices.SortFunc(user.Permissions, func(a, b Permission) int {
		return strings.Compare(a.Prefix, b.Prefix)
	})
	var dedup []Permission
	for i, v := range user.Permissions {
		if i == 0 || v.Prefix != user.Permissions[i-1].Prefix {
			dedup = append(dedup, v)
		}
	}
	user.Permissions = dedup
	return db.storeJSON(&user, "users", username)
}

func (db *DB) CreateUser(name string, admin bool) error {
	u := User{
		Name:        name,
		Admin:       admin,
		Active:      true,
		Permissions: []Permission{},
	}

	if admin {
		u.Permissions = append(u.Permissions, Permission{
			Prefix:  "/",
			Reader:  true,
			Writer:  true,
			Execute: true,
		})
	} else {
		u.Permissions = append(u.Permissions, Permission{
			Prefix:  "/home/" + name + "/",
			Reader:  true,
			Writer:  true,
			Execute: false,
		})
	}

	return db.storeJSON(&u, "users", name)
}

func (db *DB) FindUser(name string) (*User, error) {
	var u User
	err := db.loadJSON(&u, "users", name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
