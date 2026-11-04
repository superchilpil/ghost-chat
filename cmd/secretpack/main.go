package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"bytes"
)

var partA = [32]byte{0x71,0x2a,0x93,0x44,0x18,0xd0,0x6f,0x35,0xa1,0x5c,0xe2,0x09,0x87,0x3b,0xc4,0x62,0x10,0xfa,0x56,0x8d,0x23,0xb7,0x49,0xce,0x95,0x04,0x6a,0xdd,0x31,0x8e,0xf0,0x5b}
var partB = [32]byte{0x0c,0xb1,0x27,0xe8,0x63,0x19,0xa4,0x72,0xde,0x40,0x8b,0xf5,0x2a,0x96,0x01,0xcd,0x7f,0x33,0xe0,0x5a,0xb8,0x14,0x69,0x92,0x06,0xca,0x3d,0x81,0xf7,0x25,0x4e,0xa9}

func key() []byte { raw:=make([]byte,32); for i:=range raw { raw[i]=partA[i]^partB[i] }; sum:=sha256.Sum256(raw); return sum[:] }
func encrypt(value string)(string,string,error){
	block,err:=aes.NewCipher(key()); if err!=nil{return "","",err}
	gcm,err:=cipher.NewGCM(block); if err!=nil{return "","",err}
	nonce:=make([]byte,gcm.NonceSize()); if _,err=rand.Read(nonce);err!=nil{return "","",err}
	ciphertext:=gcm.Seal(nil,nonce,[]byte(value),nil)
	return base64.StdEncoding.EncodeToString(ciphertext),base64.StdEncoding.EncodeToString(nonce),nil
}
func main(){
	api:=os.Getenv("YOUTUBE_API_KEY"); password:=os.Getenv("YOUTUBE_API_BYPASS_PASSWORD")
	if len(os.Args)>1 && os.Args[1]=="-verify" {
		if len(os.Args)!=3 || api=="" || password=="" { fmt.Fprintln(os.Stderr,"verify requires executable path and both credentials"); os.Exit(2) }
		b,err:=os.ReadFile(os.Args[2]); if err!=nil{panic(err)}
		if bytes.Contains(b,[]byte(api)) || bytes.Contains(b,[]byte(password)) { fmt.Fprintln(os.Stderr,"plaintext credential found in executable"); os.Exit(1) }
		fmt.Println("Credential plaintext scan passed."); return
	}
	if api=="" || password=="" { fmt.Fprintln(os.Stderr,"YOUTUBE_API_KEY and YOUTUBE_API_BYPASS_PASSWORD are required"); os.Exit(2) }
	encAPI,nonceAPI,err:=encrypt(api); if err!=nil{panic(err)}
	encPassword,noncePassword,err:=encrypt(password); if err!=nil{panic(err)}
	source:=fmt.Sprintf("package buildconfig\nfunc init(){ encryptedAPIKey=%q; apiNonce=%q; encryptedBypassPassword=%q; bypassNonce=%q }\n",encAPI,nonceAPI,encPassword,noncePassword)
	if err:=os.WriteFile("internal/buildconfig/secrets_generated.go",[]byte(source),0600);err!=nil{panic(err)}
}
