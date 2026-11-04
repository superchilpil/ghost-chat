package buildconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

var encryptedAPIKey, encryptedBypassPassword, apiNonce, bypassNonce, expectedDigest string
var apiKeyValue, bypassPasswordValue string

func derive(digest []byte, label byte) []byte {
	salt := []byte{0x47, 0x43, 0x48, 0x54, 0x2d, 0x31}
	extract := hmac.New(sha256.New, salt)
	extract.Write(digest)
	prk := extract.Sum(nil)
	expand := hmac.New(sha256.New, prk)
	expand.Write([]byte{label, 0x01})
	return expand.Sum(nil)
}

func decrypt(value, nonce string, key []byte) (string, error) {
	if value == "" || nonce == "" {
		return "", errors.New("invalid data")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	iv, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(plaintext)), nil
}

func Initialize(asset []byte) error {
	if len(asset) == 0 || expectedDigest == "" {
		return errors.New("invalid state")
	}
	digest := sha256.Sum256(asset)
	expected, err := base64.StdEncoding.DecodeString(expectedDigest)
	if err != nil || !hmac.Equal(digest[:], expected) {
		return errors.New("invalid state")
	}
	api, err := decrypt(encryptedAPIKey, apiNonce, derive(digest[:], 0x41))
	if err != nil || api == "" {
		return errors.New("invalid state")
	}
	password, err := decrypt(encryptedBypassPassword, bypassNonce, derive(digest[:], 0x42))
	if err != nil || password == "" {
		return errors.New("invalid state")
	}
	apiKeyValue = api
	bypassPasswordValue = password
	return nil
}

func YouTubeAPIKey() string {
	return apiKeyValue
}

func YouTubeAPIBypassPassword() string {
	return bypassPasswordValue
}
