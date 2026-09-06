// Command piicli encrypts, decrypts, or blind-indexes a single value with
// go-sdk's crypto package, for preparing or inspecting database rows by hand
// without going through the API. Imports only go-sdk — no app-specific
// normalization or business rules; the caller passes exactly the value to
// encrypt/hash.
//
// Keys come from the same env vars the app reads them from — no separate
// way to pass secrets:
//
//	GUEST_PII_ENCRYPTION_KEY   standard-base64 AES-256-GCM key (32 bytes decoded)
//	GUEST_PII_BLIND_INDEX_KEY  standard-base64 HMAC-SHA256 key (>=16 bytes decoded)
//
// Usage:
//
//	piicli encrypt <value>       prints ciphertext and hash (for a manual INSERT)
//	piicli decrypt <ciphertext>  prints the plaintext value
//	piicli hash <value>          prints just the blind-index hash
package main

import (
	"fmt"
	"os"

	"github.com/biairmal/go-sdk/lib/crypto"
)

func main() {
	if len(os.Args) != 3 {
		usage()
	}
	op, value := os.Args[1], os.Args[2]

	enc := newEncryptor()
	switch op {
	case "encrypt":
		ciphertext, err := enc.Encrypt([]byte(value))
		fatalIf(err)
		fmt.Println("ciphertext: " + ciphertext)
		fmt.Println("hash:       " + enc.BlindIndex(value))

	case "decrypt":
		plaintext, err := enc.Decrypt(value)
		fatalIf(err)
		fmt.Println(string(plaintext))

	case "hash":
		fmt.Println(enc.BlindIndex(value))

	default:
		usage()
	}
}

// newEncryptor builds a crypto.Encryptor from the env vars documented in
// this file's package comment, exiting with a clear message if either is
// missing or invalid.
func newEncryptor() *crypto.Encryptor {
	cfg := crypto.Config{
		EncryptionKey: os.Getenv("GUEST_PII_ENCRYPTION_KEY"),
		BlindIndexKey: os.Getenv("GUEST_PII_BLIND_INDEX_KEY"),
	}
	enc, err := crypto.New(cfg)
	fatalIf(err)
	return enc
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "piicli: "+err.Error())
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `piicli: encrypt/decrypt/hash a value with the app's PII keys

Usage:
  piicli encrypt <value>       prints ciphertext and hash (for a manual INSERT)
  piicli decrypt <ciphertext>  prints the plaintext value
  piicli hash <value>          prints just the blind-index hash

Requires GUEST_PII_ENCRYPTION_KEY and GUEST_PII_BLIND_INDEX_KEY in the environment
(the same ones configured for the running app — see .env.example).`)
	os.Exit(2)
}
