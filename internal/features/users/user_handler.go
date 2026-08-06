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

// userListConfig declares the allow-listed sort/filter fields for user list
// queries. Pagination (page/size/max size) is not set here, so it falls back
// to the shared defaults in internal/core/query.
var userListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "tenant_id", "email", "role_id", "is_tenant_master", "created_at", "updated_at"},
	AllowedFilterFields: []string{"tenant_id", "email", "role_id", "is_tenant_master"},
}

// NewUserHandler returns a UserHandler that uses the given service and
// validator. Both are interfaces, allowing easy testing and substitution.
func NewUserHandler(service UserService, validator validation.Validator) *UserHandler {
	return &UserHandler{service: service, validator: validator}
}

// List handles GET /users with query parameters.
//
// Query format: tenant_id=...&page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// List godoc
//
//	@Summary		List users
//	@Description	Returns a paginated list of users. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (tenant_id, email, role_id, is_tenant_master).
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			page			query		int		false	"Page number (1-based)"
//	@Param			size			query		int		false	"Page size (default 20, max 100)"
//	@Param			sort			query		string	false	"Sort: field,dir (e.g. sort=email,ASC&sort=id,DESC)"
//	@Param			tenant_id		query		string	false	"Filter by tenant ID (exact match)"
//	@Param			email			query		string	false	"Filter by email (exact match)"
//	@Param			role_id			query		string	false	"Filter by role ID (exact match)"
//	@Param			is_tenant_master	query	bool	false	"Filter by tenant-master flag (exact match)"
//	@Success		200		{object}	common.PageResponse[users.User]
//	@Failure		400		{object}	object	"Invalid query (e.g. invalid sort field)"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/users [get]
func (h *UserHandler) List(r *http.Request) (any, error) {
	params, err := query.ParseListParams(r.URL.Query(), userListConfig)
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
//	@Description	Returns a single user by UUID.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"User UUID"
//	@Success		200	{object}	users.User
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"User not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Router			/api/v1/users/{id} [get]
func (h *UserHandler) GetByID(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid user id")
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
//	@Description	Creates a new user scoped to a tenant. The password is hashed before storage and never returned.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			body	body		users.CreateInput	true	"User payload"
//	@Success		201		{object}	users.User
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists)"
//	@Failure		422		{object}	object	"Unprocessable entity"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/users [post]
func (h *UserHandler) Create(r *http.Request) (any, error) {
	var body CreateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
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
//	@Description	Updates an existing user by ID. Only provided fields are applied (partial update); tenant_id is immutable.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"User UUID"
//	@Param			body	body		users.UpdateInput	true	"Fields to update"
//	@Success		200		{object}	users.User
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		404		{object}	object	"User not found"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists)"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/users/{id} [put]
func (h *UserHandler) Update(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid user id")
	}
	var body UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
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
//	@Description	Soft-deletes a user by ID.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"User UUID"
//	@Success		204	"No content"
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"User not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Router			/api/v1/users/{id} [delete]
func (h *UserHandler) Delete(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid user id")
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}
