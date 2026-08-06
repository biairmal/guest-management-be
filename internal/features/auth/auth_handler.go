package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/response"

	"github.com/biairmal/guest-management-be/internal/core/validation"
)

// Handler exposes HTTP handlers for login and token refresh.
type Handler struct {
	service   Service
	validator validation.Validator
}

// NewHandler returns an Handler that uses the given service and validator.
func NewHandler(service Service, validator validation.Validator) *Handler {
	return &Handler{service: service, validator: validator}
}

// decodeValidateCall is the shared decode → validate → call → envelope
// sequence behind both Login and Refresh, which differ only in request/
// response type and which service method they call.
func decodeValidateCall[TIn, TOut any](
	r *http.Request, validator validation.Validator, call func(context.Context, TIn) (TOut, error),
) (any, error) {
	var body TIn
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := validator.Struct(body); err != nil {
		return nil, err
	}
	out, err := call(r.Context(), body)
	if err != nil {
		return nil, err
	}
	return response.OK(out), nil
}

// Login handles POST /auth/login.
//
// Login godoc
//
//	@Summary		Login
//	@Description	Verifies email and password, and issues an access + refresh token pair.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		auth.LoginInput	true	"Credentials"
//	@Success		200		{object}	auth.TokenPair
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		401		{object}	object	"Invalid email or password"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/auth/login [post]
func (h *Handler) Login(r *http.Request) (any, error) {
	return decodeValidateCall(r, h.validator, h.service.Login)
}

// Refresh handles POST /auth/refresh.
//
// Refresh godoc
//
//	@Summary		Refresh access token
//	@Description	Exchanges a valid refresh token for a new access + refresh token pair.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		auth.RefreshInput	true	"Refresh token"
//	@Success		200		{object}	auth.TokenPair
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		401		{object}	object	"Invalid, expired, or non-refresh token"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/auth/refresh [post]
func (h *Handler) Refresh(r *http.Request) (any, error) {
	return decodeValidateCall(r, h.validator, h.service.Refresh)
}
