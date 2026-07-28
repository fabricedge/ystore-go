package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	SaltLen   = 16
	NonceLen  = 12
	KeyLen    = 32
)

var (
	ArgonTime    = uint32(3)
	ArgonMemory  = uint32(64 * 1024)
	ArgonThreads = uint8(4)
)

func Encrypt(plaintext []byte, password string) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("password must not be empty")
	}

	salt := make([]byte, SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, KeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}

	nonce := make([]byte, NonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, SaltLen+NonceLen+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	return out, nil
}

func Decrypt(encrypted []byte, password string) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("password must not be empty")
	}

	if len(encrypted) < SaltLen+NonceLen {
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := encrypted[:SaltLen]
	nonce := encrypted[SaltLen : SaltLen+NonceLen]
	ciphertext := encrypted[SaltLen+NonceLen:]

	key := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, KeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w (wrong password?)", err)
	}

	return plaintext, nil
}
