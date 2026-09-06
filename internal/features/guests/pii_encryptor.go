package guests

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/guests/mock_pii_encryptor.go -package=mockguests github.com/biairmal/guest-management-be/internal/features/guests PIIEncryptor

// PIIEncryptor abstracts field-level encryption for guest PII (email, phone)
// so the underlying scheme can change without touching guest_repository.go
// or guest_service.go — the product owner may swap it for a different
// cipher, a KMS-backed implementation, or a remote encryption service later.
// internal/app wires the one concrete implementation this phase (a thin
// adapter over go-sdk's crypto package); this package only ever depends on
// the interface.
//
// Encrypt/Decrypt are for storage/display and are expected to be
// non-deterministic (the same plaintext may produce different ciphertext
// each call) for semantic security. BlindIndex is deliberately separate: a
// deterministic digest used only for exact-match search — see
// guest_repository.go's email_hash/phone_hash usage for why Encrypt/Decrypt
// alone can't support search (comparing non-deterministic ciphertext can
// never match).
type PIIEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
	BlindIndex(value string) (string, error)
}
