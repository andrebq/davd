package config

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/nacl/secretbox"
)

var (
	ErrNoSuchKey = fmt.Errorf("no such key")
)

func (db *DB) pathToKey(parts ...string) string {
	path := filepath.Join(db.abs, filepath.FromSlash(path.Clean(path.Join(parts...))))
	path = strings.TrimSuffix(path, ".json")
	return fmt.Sprintf("%s.json", path)
}

func (db *DB) loadJSON(v interface{}, parts ...string) error {
	path := db.pathToKey(parts...)
	return loadJSONFile(v, path)
}

func (db *DB) storeJSON(v interface{}, parts ...string) error {
	// storeJSON is not atomic... fix this eventually!
	// could use a temp file with O_EXCL and then rename
	path := db.pathToKey(parts...)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	enc := json.NewEncoder(fd)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (db *DB) storeEncryptedJSON(v interface{}, key []byte, parts ...string) error {
	// storeEncryptedJSON is not atomic... fix this eventually!
	extended := deriveKey(db.storageEncryptionKey[:], []byte("encrypted_json"), key)
	jsonData, err := json.Marshal(v)
	if err != nil {
		return err
	}
	encData := encryptBuffer(&extended, jsonData)
	path := db.pathToKey(parts...)
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fd.Write(encData)
	return err
}

func (db *DB) loadEncryptedJSON(v interface{}, key []byte, parts ...string) error {
	extended := deriveKey(db.storageEncryptionKey[:], []byte("encrypted_json"), key)
	path := db.pathToKey(parts...)
	encData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoSuchKey
		}
		return err
	}
	decData, err := decryptBuffer(&extended, encData)
	if err != nil {
		return err
	}
	return json.Unmarshal(decData, v)
}

func loadJSONFile(v interface{}, path string) error {
	fd, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoSuchKey
		}
		return err
	}
	defer fd.Close()
	dec := json.NewDecoder(fd)
	return dec.Decode(v)
}

func encryptBuffer(key *[32]byte, plaintext []byte) []byte {
	var nonce [24]byte
	_, err := rand.Read(nonce[:])
	if err != nil {
		panic("FATAL RUNTIME ERROR: unable to read from crypto/rand")
	}
	return append(nonce[:], secretbox.Seal(nil, plaintext, &nonce, key)...)
}

func decryptBuffer(key *[32]byte, ciphertext []byte) ([]byte, error) {
	var nonce [24]byte
	if len(ciphertext) < 24 {
		return nil, fmt.Errorf("ciphertext too short")
	}
	copy(nonce[:], ciphertext[:24])
	decrypted, ok := secretbox.Open(nil, ciphertext[24:], &nonce, key)
	if !ok {
		return nil, fmt.Errorf("decryption failed")
	}
	return decrypted, nil
}
