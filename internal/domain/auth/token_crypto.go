package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

func encryptToken(key, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := newCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decryptToken(key, encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	block, err := newCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", errors.New("invalid token payload")
	}
	nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func newCipher(key string) (cipher.Block, error) {
	raw := []byte(key)
	switch len(raw) {
	case 16, 24, 32:
		return aes.NewCipher(raw)
	default:
		return nil, errors.New("token encryption key must be 16, 24, or 32 bytes")
	}
}
