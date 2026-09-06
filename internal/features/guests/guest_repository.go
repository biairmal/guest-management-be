package guests

import (
	"context"
	"fmt"
	"strings"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const guestsTable = "guests"

// guestColumns are the columns selected on reads (GetByID, List). Ticket is
// not a column (see guest_model.go) and is excluded.
var guestColumns = []string{
	"id", "event_id", "name", "email", "phone", "email_hash", "phone_hash",
	"ticket_type_id", "invitation_token", "rsvp_status", "ticket_id",
	"created_at", "updated_at", "deleted_at",
}

// guestRepository wraps the generic soft-delete-aware repository with
// transparent field-level encryption for email/phone (see PIIEncryptor):
// Create/Update encrypt those fields immediately before delegating,
// GetByID/List decrypt them immediately after. Every other method
// (Delete/Count/Exists) is inherited unchanged from the embedded
// repository.Repository.
type guestRepository struct {
	repository.Repository[Guest, uuid.UUID]
	encryptor PIIEncryptor
}

// NewGuestRepository returns a soft-delete-aware, PII-encrypting repository
// for guests. TID is uuid.UUID — kept typed all the way through the service
// layer.
func NewGuestRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions, encryptor PIIEncryptor,
) repository.Repository[Guest, uuid.UUID] {
	base := corerepository.NewRepository[Guest, uuid.UUID](log, db, guestsTable, guestColumns, cacheOpts)
	return &guestRepository{Repository: base, encryptor: encryptor}
}

// Create encrypts email/phone (and computes their blind-index hashes)
// immediately before delegating, then restores entity's plaintext view so
// the caller (GuestService) never sees ciphertext.
func (r *guestRepository) Create(ctx context.Context, entity *Guest) error {
	plainEmail, plainPhone := entity.Email, entity.Phone
	if err := r.encryptPII(entity); err != nil {
		return err
	}
	err := r.Repository.Create(ctx, entity)
	entity.Email, entity.Phone = plainEmail, plainPhone
	return err
}

// Update encrypts email/phone immediately before delegating, then restores
// entity's plaintext view, mirroring Create.
func (r *guestRepository) Update(ctx context.Context, id uuid.UUID, entity *Guest) error {
	plainEmail, plainPhone := entity.Email, entity.Phone
	if err := r.encryptPII(entity); err != nil {
		return err
	}
	err := r.Repository.Update(ctx, id, entity)
	entity.Email, entity.Phone = plainEmail, plainPhone
	return err
}

// GetByID decrypts the returned row's email/phone before returning it.
func (r *guestRepository) GetByID(ctx context.Context, id uuid.UUID) (*Guest, error) {
	entity, err := r.Repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.decryptPII(entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// List decrypts every returned row's email/phone before returning them.
func (r *guestRepository) List(ctx context.Context, opts *repository.ListOptions) ([]*Guest, int64, error) {
	items, total, err := r.Repository.List(ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	for _, item := range items {
		if err := r.decryptPII(item); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

// encryptPII replaces entity's Email/Phone with ciphertext and computes
// EmailHash/PhoneHash from the normalized plaintext, mutating entity in
// place. Phone is only processed when present.
func (r *guestRepository) encryptPII(entity *Guest) error {
	emailCipher, err := r.encryptor.Encrypt(entity.Email)
	if err != nil {
		return fmt.Errorf("guests: encrypt email: %w", err)
	}
	emailHash, err := r.encryptor.BlindIndex(normalizeEmail(entity.Email))
	if err != nil {
		return fmt.Errorf("guests: blind index email: %w", err)
	}
	entity.Email = emailCipher
	entity.EmailHash = emailHash

	if entity.Phone == nil {
		return nil
	}
	phoneCipher, err := r.encryptor.Encrypt(*entity.Phone)
	if err != nil {
		return fmt.Errorf("guests: encrypt phone: %w", err)
	}
	phoneHash, err := r.encryptor.BlindIndex(normalizePhone(*entity.Phone))
	if err != nil {
		return fmt.Errorf("guests: blind index phone: %w", err)
	}
	entity.Phone = &phoneCipher
	entity.PhoneHash = &phoneHash
	return nil
}

// decryptPII replaces entity's Email/Phone ciphertext with plaintext,
// mutating entity in place. Phone is only processed when present.
func (r *guestRepository) decryptPII(entity *Guest) error {
	email, err := r.encryptor.Decrypt(entity.Email)
	if err != nil {
		return fmt.Errorf("guests: decrypt email: %w", err)
	}
	entity.Email = email

	if entity.Phone == nil {
		return nil
	}
	phone, err := r.encryptor.Decrypt(*entity.Phone)
	if err != nil {
		return fmt.Errorf("guests: decrypt phone: %w", err)
	}
	entity.Phone = &phone
	return nil
}

// normalizeEmail lowercases and trims email so the blind index matches
// regardless of case/whitespace differences between writes and searches.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// normalizePhone strips everything but digits so the blind index matches
// regardless of formatting differences (spaces, dashes, country-code
// punctuation) between writes and searches.
func normalizePhone(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
