package tenants

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

// TenantHandler exposes HTTP handlers for tenant CRUD.
type TenantHandler struct {
	service   TenantService
	validator validation.Validator
}

// tenantListConfig declares the allow-listed sort/filter fields for tenant
// list queries. Pagination (page/size/max size) is not set here, so it falls
// back to the shared defaults in internal/core/query.
var tenantListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "name", "type", "created_at", "updated_at"},
	AllowedFilterFields: []string{"name", "type"},
}

// NewTenantHandler returns a TenantHandler that uses the given service and
// validator. Both are interfaces, allowing easy testing and substitution.
func NewTenantHandler(service TenantService, validator validation.Validator) *TenantHandler {
	return &TenantHandler{service: service, validator: validator}
}

// List handles GET /tenants with query parameters.
//
// Query format: name=Acme&page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// List godoc
//
//	@Summary		List tenants
//	@Description	Returns a paginated list of tenants. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name, type).
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			page	query		int		false	"Page number (1-based)"
//	@Param			size	query		int		false	"Page size (default 20, max 100)"
//	@Param			sort	query		string	false	"Sort: field,dir (e.g. sort=name,ASC&sort=id,DESC)"
//	@Param			name	query		string	false	"Filter by name (exact match)"
//	@Param			type	query		string	false	"Filter by type (exact match)"
//	@Success		200		{object}	common.PageResponse[tenants.Tenant]
//	@Failure		400		{object}	object	"Invalid query (e.g. invalid sort field)"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/tenants [get]
func (h *TenantHandler) List(r *http.Request) (any, error) {
	params, err := query.ParseListParams(r.URL.Query(), tenantListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[Tenant]
	result, err = h.service.List(r.Context(), params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /tenants/{id}.
//
// GetByID godoc
//
//	@Summary		Get tenant by ID
//	@Description	Returns a single tenant by UUID.
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Tenant UUID"
//	@Success		200	{object}	tenants.Tenant
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Tenant not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/tenants/{id} [get]
func (h *TenantHandler) GetByID(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid tenant id")
	}
	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /tenants.
//
// Create godoc
//
//	@Summary		Create tenant
//	@Description	Creates a new tenant. Settings and branding default to an empty object when omitted.
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			body	body		tenants.CreateInput	true	"Tenant payload"
//	@Success		201		{object}	tenants.Tenant
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists)"
//	@Failure		422		{object}	object	"Unprocessable entity"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/tenants [post]
func (h *TenantHandler) Create(r *http.Request) (any, error) {
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

// Update handles PUT /tenants/{id}.
//
// Update godoc
//
//	@Summary		Update tenant
//	@Description	Updates an existing tenant by ID. Only provided fields are applied (partial update).
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Tenant UUID"
//	@Param			body	body		tenants.UpdateInput	true	"Fields to update"
//	@Success		200		{object}	tenants.Tenant
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		404		{object}	object	"Tenant not found"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/tenants/{id} [put]
func (h *TenantHandler) Update(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid tenant id")
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

// Delete handles DELETE /tenants/{id}.
//
// Delete godoc
//
//	@Summary		Delete tenant
//	@Description	Soft-deletes a tenant by ID.
//	@Tags			tenants
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Tenant UUID"
//	@Success		204	"No content"
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Tenant not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/tenants/{id} [delete]
func (h *TenantHandler) Delete(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid tenant id")
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}
