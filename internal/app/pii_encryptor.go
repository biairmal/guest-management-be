package app

import (
	sdkcrypto "github.com/biairmal/go-sdk/lib/crypto"
	"github.com/biairmal/guest-management-be/internal/features/guests"
)

// cryptoPIIEncryptor is the guests.PIIEncryptor implementation: a thin
// adapter over go-sdk's crypto.Encryptor (AES-256-GCM + HMAC-SHA256 blind
// index). This is the only file in the app that imports go-sdk/crypto —
// guest_repository.go and guest_service.go depend only on the
// guests.PIIEncryptor interface, so swapping the scheme later (a different
// cipher, a KMS-backed implementation, a remote encryption service) is a
// change to this one file plus its wiring in repository.go, nothing in
// guests.
type cryptoPIIEncryptor struct {
	enc *sdkcrypto.Encryptor
}

// newCryptoPIIEncryptor builds the guests.PIIEncryptor from cfg (the app's
// crypto.EncryptionKey/BlindIndexKey — see internal/config.Config.Crypto).
func newCryptoPIIEncryptor(cfg sdkcrypto.Config) (guests.PIIEncryptor, error) {
	enc, err := sdkcrypto.New(cfg)
	if err != nil {
		return nil, err
	}
	return &cryptoPIIEncryptor{enc: enc}, nil
}

// Encrypt returns a base64-encoded AES-256-GCM ciphertext of plaintext.
func (e *cryptoPIIEncryptor) Encrypt(plaintext string) (string, error) {
	return e.enc.Encrypt([]byte(plaintext))
}

// Decrypt reverses Encrypt.
func (e *cryptoPIIEncryptor) Decrypt(ciphertext string) (string, error) {
	plaintext, err := e.enc.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// BlindIndex returns a deterministic, hex-encoded HMAC-SHA256 of value.
// go-sdk's Encryptor.BlindIndex never errors (see its doc comment); this
// still returns an error to satisfy guests.PIIEncryptor's interface, which
// stays general enough for a future implementation that could fail (e.g. a
// remote/KMS-backed one).
func (e *cryptoPIIEncryptor) BlindIndex(value string) (string, error) {
	return e.enc.BlindIndex(value), nil
}
