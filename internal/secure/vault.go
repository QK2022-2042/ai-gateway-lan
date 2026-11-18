package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Vault encrypts upstream provider credentials at rest with AES-256-GCM.
// The key lives in a 0600 file next to the database and never in the DB.
type Vault struct {
	aead cipher.AEAD
}

const vaultPrefix = "v1:"

func LoadOrCreateVault(path string) (*Vault, error) {
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, key, 0o600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key %s must be exactly 32 bytes", path)
	}
	return NewVault(key)
}

func NewVault(key []byte) (*Vault, error) {
	if len(key) != 32 {
		return nil, errors.New("vault key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

func (v *Vault) Encrypt(plain []byte) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := v.aead.Seal(nonce, nonce, plain, nil)
	return vaultPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (v *Vault) Decrypt(enc string) ([]byte, error) {
	if enc == "" {
		return nil, nil
	}
	if !strings.HasPrefix(enc, vaultPrefix) {
		return nil, errors.New("unsupported ciphertext format")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(enc, vaultPrefix))
	if err != nil {
		return nil, err
	}
	ns := v.aead.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return v.aead.Open(nil, raw[:ns], raw[ns:], nil)
}

func (v *Vault) EncryptStrings(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return v.Encrypt(b)
}

func (v *Vault) DecryptStrings(enc string) ([]string, error) {
	b, err := v.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	out := []string{}
	if len(b) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
