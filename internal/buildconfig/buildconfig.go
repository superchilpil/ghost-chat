package buildconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var encryptedAPIKey, encryptedBypassPassword, apiNonce, bypassNonce, expectedDigest string
var apiKeyValue, bypassPasswordValue string

func derive(digest []byte, label byte) []byte {
	mac := hmac.New(sha256.New, digest)
	mac.Write([]byte{label, 0x01})
	return mac.Sum(nil)
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
	ciphertext, err := hex.DecodeString(value)
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(nonce)
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
	expected, err := hex.DecodeString(expectedDigest)
	if err != nil || len(expected) != sha256.Size || !hmac.Equal(digest[:], expected) {
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
