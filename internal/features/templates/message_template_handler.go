package templates

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

// MessageTemplateHandler exposes HTTP handlers for message template CRUD.
type MessageTemplateHandler struct {
	service   MessageTemplateService
	validator validation.Validator
}

// NewMessageTemplateHandler returns a MessageTemplateHandler that uses the
// given service and validator. Both are interfaces, allowing easy testing
// and substitution.
func NewMessageTemplateHandler(service MessageTemplateService, validator validation.Validator) *MessageTemplateHandler {
	return &MessageTemplateHandler{service: service, validator: validator}
}

// List handles GET /message-templates with query parameters.
//
// Query format: name=Invitation&channel=email&page=1&size=20&sort=column1,DESC
//
// List godoc
//
//	@Summary		List message templates
//	@Description	Returns a paginated list of message templates. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name, source, tenant_id, event_id, channel).
//	@Tags			message-templates
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=name,ASC&sort=id,DESC)"
//	@Param			name		query		string	false	"Filter by name (exact match)"
//	@Param			source		query		string	false	"Filter by source (exact match)"
//	@Param			channel		query		string	false	"Filter by channel (exact match)"
//	@Success		200			{object}	common.PageResponse[templates.MessageTemplate]
//	@Failure		400			{object}	object	"Invalid query (e.g. invalid sort field)"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/message-templates [get]
func (h *MessageTemplateHandler) List(r *http.Request) (any, error) {
	params, err := query.ParseListParams(r.URL.Query(), MessageTemplateListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[MessageTemplate]
	result, err = h.service.List(r.Context(), params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /message-templates/{id}.
//
// GetByID godoc
//
//	@Summary		Get message template by ID
//	@Description	Returns a single message template by UUID.
//	@Tags			message-templates
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Message template UUID"
//	@Success		200	{object}	templates.MessageTemplate
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Message template not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/message-templates/{id} [get]
func (h *MessageTemplateHandler) GetByID(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid message template id")
	}
	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /message-templates.
//
// Create godoc
//
//	@Summary		Create message template
//	@Description	Creates a new message template. Source must be "app", "tenant", or "event"; tenant_id/event_id required accordingly. Subject is required for "email" channel and must be empty for "whatsapp".
//	@Tags			message-templates
//	@Accept			json
//	@Produce		json
//	@Param			body	body		templates.CreateInput	true	"Message template payload"
//	@Success		201		{object}	templates.MessageTemplate
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists for this scope, name, and channel)"
//	@Failure		422		{object}	object	"Unprocessable entity"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/message-templates [post]
func (h *MessageTemplateHandler) Create(r *http.Request) (any, error) {
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

// Update handles PUT /message-templates/{id}.
//
// Update godoc
//
//	@Summary		Update message template
//	@Description	Updates an existing message template by ID. Only provided fields are applied (partial update); the source/channel invariants re-apply to the resulting record.
//	@Tags			message-templates
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Message template UUID"
//	@Param			body	body		templates.UpdateInput	true	"Fields to update"
//	@Success		200		{object}	templates.MessageTemplate
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		404		{object}	object	"Message template not found"
//	@Failure		409		{object}	object	"Conflict"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/message-templates/{id} [put]
func (h *MessageTemplateHandler) Update(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid message template id")
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

// Delete handles DELETE /message-templates/{id}.
//
// Delete godoc
//
//	@Summary		Delete message template
//	@Description	Soft-deletes a message template by ID.
//	@Tags			message-templates
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Message template UUID"
//	@Success		204	"No content"
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Message template not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/message-templates/{id} [delete]
func (h *MessageTemplateHandler) Delete(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid message template id")
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}
