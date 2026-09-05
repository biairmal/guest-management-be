package workflowsteptemplate

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

// Handler exposes HTTP handlers for category-scoped workflow step template CRUD.
type Handler struct {
	service   Service
	validator validation.Validator
}

// NewHandler returns a Handler that
// uses the given service and validator. Both are interfaces, allowing easy
// testing and substitution.
func NewHandler(
	service Service, validator validation.Validator,
) *Handler {
	return &Handler{service: service, validator: validator}
}

// categoryIDFromPath parses the {category_id} path parameter.
func categoryIDFromPath(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "category_id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid event category id")
	}
	return id, nil
}

// List handles GET /event-categories/{category_id}/workflow-step-templates with query parameters.
//
// List godoc
//
//	@Summary		List workflow step templates
//	@Description	Returns a paginated list of workflow step templates for an event category. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name).
//	@Tags			workflow-step-templates
//	@Accept			json
//	@Produce		json
//	@Param			category_id	path		string	true	"Event category UUID"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=order_index,ASC)"
//	@Param			name		query		string	false	"Filter by name (exact match)"
//	@Success		200			{object}	common.PageResponse[workflowsteptemplate.WorkflowStepTemplate]
//	@Failure		400			{object}	object	"Invalid category id or query"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/event-categories/{category_id}/workflow-step-templates [get]
func (h *Handler) List(r *http.Request) (any, error) {
	categoryID, err := categoryIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), WorkflowStepTemplateListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[WorkflowStepTemplate]
	result, err = h.service.List(r.Context(), categoryID, params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /event-categories/{category_id}/workflow-step-templates/{id}.
//
// GetByID godoc
//
//	@Summary		Get workflow step template by ID
//	@Description	Returns a single workflow step template by UUID, scoped to its event category.
//	@Tags			workflow-step-templates
//	@Accept			json
//	@Produce		json
//	@Param			category_id	path		string	true	"Event category UUID"
//	@Param			id			path		string	true	"Workflow step template UUID"
//	@Success		200			{object}	workflowsteptemplate.WorkflowStepTemplate
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		404			{object}	object	"Workflow step template not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/event-categories/{category_id}/workflow-step-templates/{id} [get]
func (h *Handler) GetByID(r *http.Request) (any, error) {
	categoryID, err := categoryIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid workflow step template id")
	}
	entity, err := h.service.GetByID(r.Context(), categoryID, id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /event-categories/{category_id}/workflow-step-templates.
//
// Create godoc
//
//	@Summary		Create workflow step template
//	@Description	Creates a new workflow step template under an event category. order_index must be unique per category.
//	@Tags			workflow-step-templates
//	@Accept			json
//	@Produce		json
//	@Param			category_id	path		string									true	"Event category UUID"
//	@Param			body		body		workflowsteptemplate.CreateWorkflowStepTemplateInput	true	"Workflow step template payload"
//	@Success		201			{object}	workflowsteptemplate.WorkflowStepTemplate
//	@Failure		400			{object}	object	"Invalid request body or validation error"
//	@Failure		409			{object}	object	"Conflict (order_index already used for this category)"
//	@Failure		422			{object}	object	"Unprocessable entity"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/event-categories/{category_id}/workflow-step-templates [post]
func (h *Handler) Create(r *http.Request) (any, error) {
	categoryID, err := categoryIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body CreateWorkflowStepTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.Create(r.Context(), categoryID, body)
	if err != nil {
		return nil, err
	}
	return response.Created(entity), nil
}

// Update handles PUT /event-categories/{category_id}/workflow-step-templates/{id}.
//
// Update godoc
//
//	@Summary		Update workflow step template
//	@Description	Updates an existing workflow step template by ID. Only provided fields are applied (partial update); category_id is immutable.
//	@Tags			workflow-step-templates
//	@Accept			json
//	@Produce		json
//	@Param			category_id	path		string									true	"Event category UUID"
//	@Param			id			path		string									true	"Workflow step template UUID"
//	@Param			body		body		workflowsteptemplate.UpdateWorkflowStepTemplateInput	true	"Fields to update"
//	@Success		200			{object}	workflowsteptemplate.WorkflowStepTemplate
//	@Failure		400			{object}	object	"Invalid ID or request body"
//	@Failure		404			{object}	object	"Workflow step template not found"
//	@Failure		409			{object}	object	"Conflict (order_index already used for this category)"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/event-categories/{category_id}/workflow-step-templates/{id} [put]
func (h *Handler) Update(r *http.Request) (any, error) {
	categoryID, err := categoryIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid workflow step template id")
	}
	var body UpdateWorkflowStepTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.Update(r.Context(), categoryID, id, body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Delete handles DELETE /event-categories/{category_id}/workflow-step-templates/{id}.
//
// Delete godoc
//
//	@Summary		Delete workflow step template
//	@Description	Soft-deletes a workflow step template by ID, scoped to its event category.
//	@Tags			workflow-step-templates
//	@Accept			json
//	@Produce		json
//	@Param			category_id	path	string	true	"Event category UUID"
//	@Param			id			path	string	true	"Workflow step template UUID"
//	@Success		204			"No content"
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		404			{object}	object	"Workflow step template not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/event-categories/{category_id}/workflow-step-templates/{id} [delete]
func (h *Handler) Delete(r *http.Request) (any, error) {
	categoryID, err := categoryIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid workflow step template id")
	}
	if err := h.service.Delete(r.Context(), categoryID, id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}
