// Package validation adapts go-sdk's validator behind a minimal interface
// (Struct) so handlers depend on an app-owned seam rather than importing
// go-sdk/validator directly. Validation failures surface as an *errorz.Error
// (errorz.CodeBadRequest) with per-field detail, produced by go-sdk.
package validation

import gosdkvalidator "github.com/biairmal/go-sdk/lib/validator"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/validation/mock_validator.go -package=mockvalidation github.com/biairmal/guest-management-be/internal/core/validation Validator

// Validator validates a value's fields against its `validate` struct tags.
type Validator interface {
	// Struct validates v using its struct tags. Returns nil when valid, or an
	// *errorz.Error (errorz.CodeBadRequest) with per-field messages.
	Struct(v any) error
}

// adapter implements Validator by delegating to a go-sdk validator.Validator.
type adapter struct {
	v gosdkvalidator.Validator
}

// New returns a Validator backed by go-sdk's validator package, built from
// the given config (see gosdkvalidator.Config / DefaultConfig).
func New(cfg gosdkvalidator.Config, opts ...gosdkvalidator.Option) Validator {
	return &adapter{v: gosdkvalidator.New(cfg, opts...)}
}

// Struct delegates to the underlying go-sdk validator's ValidateStruct.
func (a *adapter) Struct(v any) error {
	return a.v.ValidateStruct(v)
}
