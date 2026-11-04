package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
)

func derive(seed []byte, label byte) []byte {
	mac := hmac.New(sha256.New, seed)
	mac.Write([]byte{label, 0x01})
	return mac.Sum(nil)
}

func encrypt(value string, key []byte) (string, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

func main() {
	api := os.Getenv("YOUTUBE_API_KEY")
	password := os.Getenv("YOUTUBE_API_BYPASS_PASSWORD")
	if len(os.Args) > 1 && os.Args[1] == "-verify" {
		if len(os.Args) != 3 || api == "" || password == "" {
			fmt.Fprintln(os.Stderr, "verify requires executable path and both credentials")
			os.Exit(2)
		}
		b, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		if bytes.Contains(b, []byte(api)) || bytes.Contains(b, []byte(password)) {
			fmt.Fprintln(os.Stderr, "plaintext credential found in executable")
			os.Exit(1)
		}
		fmt.Println("Credential plaintext scan passed.")
		return
	}
	assetKey := os.Getenv("ASSET_ENCRYPTION_KEY")
	if api == "" || password == "" || assetKey == "" {
		fmt.Fprintln(os.Stderr, "required build secrets are missing")
		os.Exit(2)
	}
	asset, err := os.ReadFile("build/appicon.png")
	if err != nil || len(asset) == 0 {
		fmt.Fprintln(os.Stderr, "asset unavailable")
		os.Exit(2)
	}
	digest := sha256.Sum256(asset)
	mac := hmac.New(sha256.New, []byte(assetKey))
	mac.Write(digest[:])
	seed := mac.Sum(nil)
	encAPI, nonceAPI, err := encrypt(api, derive(seed, 0x41))
	if err != nil {
		panic(err)
	}
	encPassword, noncePassword, err := encrypt(password, derive(seed, 0x42))
	if err != nil {
		panic(err)
	}
	source := fmt.Sprintf(
		"package buildconfig\nfunc init(){ expectedDigest=%q; assetKeySeed=%q; encryptedAPIKey=%q; apiNonce=%q; encryptedBypassPassword=%q; bypassNonce=%q }\n",
		base64.StdEncoding.EncodeToString(digest[:]), base64.StdEncoding.EncodeToString(seed), encAPI, nonceAPI, encPassword, noncePassword,
	)
	if err := os.WriteFile("internal/buildconfig/secrets_generated.go", []byte(source), 0600); err != nil {
		panic(err)
	}
}
