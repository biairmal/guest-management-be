package users

import (
	"encoding/json"
	"net/http"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/core/validation"
)

// UserHandler exposes HTTP handlers for user CRUD.
type UserHandler struct {
	service   UserService
	validator validation.Validator
}

// NewUserHandler returns a UserHandler that uses the given service and
// validator. Both are interfaces, allowing easy testing and substitution.
func NewUserHandler(service UserService, validator validation.Validator) *UserHandler {
	return &UserHandler{service: service, validator: validator}
}

// parseUserID parses the "id" URL parameter as a UUID, or returns
// errorz.BadRequest — shared by every handler below that takes a user ID.
func parseUserID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid user id")
	}
	return id, nil
}

// decodeBody decodes r's JSON body into a T and runs it through the shared
// validator, or returns errorz.BadRequest / a validation error — shared by
// every handler below that takes a request body.
func decodeBody[T any](r *http.Request, v validation.Validator) (T, error) {
	var body T
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return body, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := v.Struct(body); err != nil {
		return body, err
	}
	return body, nil
}

// List handles GET /users with query parameters.
//
// Query format: page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// List godoc
//
//	@Summary		List users
//	@Description	Returns a paginated list of users in the caller's own tenant (resolved from the JWT). Query: page, size, sort=field,dir (repeatable), filter by allowed fields (email, role_id, is_tenant_master).
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			page				query		int		false	"Page number (1-based)"
//	@Param			size				query		int		false	"Page size (default 20, max 100)"
//	@Param			sort				query		string	false	"Sort: field,dir (e.g. sort=email,ASC&sort=id,DESC)"
//	@Param			email				query		string	false	"Filter by email (exact match)"
//	@Param			role_id				query		string	false	"Filter by role ID (exact match)"
//	@Param			is_tenant_master	query		bool	false	"Filter by tenant-master flag (exact match)"
//	@Success		200		{object}	common.PageResponse[users.User]
//	@Failure		400		{object}	object	"Invalid query (e.g. invalid sort field)"
//	@Failure		401		{object}	object	"Missing or invalid token"
//	@Failure		403		{object}	object	"Missing manage_users permission"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users [get]
func (h *UserHandler) List(r *http.Request) (any, error) {
	params, err := query.ParseListParams(r.URL.Query(), UserListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[User]
	result, err = h.service.List(r.Context(), params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /users/{id}.
//
// GetByID godoc
//
//	@Summary		Get user by ID
//	@Description	Returns a single user by UUID, scoped to the caller's own tenant.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"User UUID"
//	@Success		200	{object}	users.User
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		401	{object}	object	"Missing or invalid token"
//	@Failure		403	{object}	object	"Missing manage_users permission"
//	@Failure		404	{object}	object	"User not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id} [get]
func (h *UserHandler) GetByID(r *http.Request) (any, error) {
	id, err := parseUserID(r)
	if err != nil {
		return nil, err
	}
	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /users.
//
// Create godoc
//
//	@Summary		Create user
//	@Description	Creates a new user scoped to the caller's own tenant (resolved from the JWT). The password is hashed before storage and never returned; must_change_password always starts true.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			body	body		users.CreateInput	true	"User payload"
//	@Success		201		{object}	users.User
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		401		{object}	object	"Missing or invalid token"
//	@Failure		403		{object}	object	"Missing manage_users permission"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists)"
//	@Failure		422		{object}	object	"Unprocessable entity"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users [post]
func (h *UserHandler) Create(r *http.Request) (any, error) {
	body, err := decodeBody[CreateInput](r, h.validator)
	if err != nil {
		return nil, err
	}
	entity, err := h.service.Create(r.Context(), body)
	if err != nil {
		return nil, err
	}
	return response.Created(entity), nil
}

// Update handles PUT /users/{id}.
//
// Update godoc
//
//	@Summary		Update user
//	@Description	Updates an existing user by ID, scoped to the caller's own tenant. Only provided fields are applied (partial update); tenant_id is immutable and password changes go through POST /{id}/password.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"User UUID"
//	@Param			body	body		users.UpdateInput	true	"Fields to update"
//	@Success		200		{object}	users.User
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		401		{object}	object	"Missing or invalid token"
//	@Failure		403		{object}	object	"Missing manage_users permission"
//	@Failure		404		{object}	object	"User not found"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists)"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id} [put]
func (h *UserHandler) Update(r *http.Request) (any, error) {
	id, err := parseUserID(r)
	if err != nil {
		return nil, err
	}
	body, err := decodeBody[UpdateInput](r, h.validator)
	if err != nil {
		return nil, err
	}
	entity, err := h.service.Update(r.Context(), id, body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Delete handles DELETE /users/{id}.
//
// Delete godoc
//
//	@Summary		Delete user
//	@Description	Soft-deletes a user by ID, scoped to the caller's own tenant.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"User UUID"
//	@Success		204	"No content"
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		401	{object}	object	"Missing or invalid token"
//	@Failure		403	{object}	object	"Missing manage_users permission"
//	@Failure		404	{object}	object	"User not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id} [delete]
func (h *UserHandler) Delete(r *http.Request) (any, error) {
	id, err := parseUserID(r)
	if err != nil {
		return nil, err
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}

// SetPassword handles POST /users/{id}/password.
//
// SetPassword godoc
//
//	@Summary		Set another user's password (admin)
//	@Description	Sets a new password for {id} and clears must_change_password. Admin-only — requires the manage_users permission. To change your own password, use POST /api/v1/users/me/password instead.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"User UUID"
//	@Param			body	body		users.SetPasswordInput	true	"New password"
//	@Success		200		{object}	users.User
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		401		{object}	object	"Missing or invalid token"
//	@Failure		403		{object}	object	"Missing manage_users permission"
//	@Failure		404		{object}	object	"User not found"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id}/password [post]
func (h *UserHandler) SetPassword(r *http.Request) (any, error) {
	id, err := parseUserID(r)
	if err != nil {
		return nil, err
	}
	body, err := decodeBody[SetPasswordInput](r, h.validator)
	if err != nil {
		return nil, err
	}
	entity, err := h.service.SetPassword(r.Context(), id, body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// SetOwnPassword handles POST /users/me/password.
//
// SetOwnPassword godoc
//
//	@Summary		Set my own password
//	@Description	Sets a new password for the caller's own account and clears must_change_password. Self-service — the target id is always resolved from the caller's access token, never a path param or request field. Requires only a valid token.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			body	body		users.SetPasswordInput	true	"New password"
//	@Success		200		{object}	users.User
//	@Failure		400		{object}	object	"Invalid request body"
//	@Failure		401		{object}	object	"Missing or invalid token"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users/me/password [post]
func (h *UserHandler) SetOwnPassword(r *http.Request) (any, error) {
	body, err := decodeBody[SetPasswordInput](r, h.validator)
	if err != nil {
		return nil, err
	}
	entity, err := h.service.SetOwnPassword(r.Context(), body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// TransferMaster handles POST /users/{id}/transfer-master.
//
// TransferMaster godoc
//
//	@Summary		Transfer tenant-master ownership
//	@Description	Transfers the caller's is_tenant_master flag to {id}, an active user in the same tenant. The caller must currently be the tenant master.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Target user UUID (new tenant master)"
//	@Success		200	{object}	users.User
//	@Failure		400	{object}	object	"Invalid ID"
//	@Failure		401	{object}	object	"Missing or invalid token"
//	@Failure		403	{object}	object	"Missing manage_users permission, or caller is not the tenant master"
//	@Failure		404	{object}	object	"Target user not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id}/transfer-master [post]
func (h *UserHandler) TransferMaster(r *http.Request) (any, error) {
	id, err := parseUserID(r)
	if err != nil {
		return nil, err
	}
	entity, err := h.service.TransferMaster(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}
