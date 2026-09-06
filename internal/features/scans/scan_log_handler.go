package scans

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

// ScanLogHandler exposes HTTP handlers for event-scoped scan recording and
// scan history.
type ScanLogHandler struct {
	service   ScanLogService
	validator validation.Validator
}

// NewScanLogHandler returns a ScanLogHandler that uses the given service and
// validator. Both are interfaces, allowing easy testing and substitution.
func NewScanLogHandler(service ScanLogService, validator validation.Validator) *ScanLogHandler {
	return &ScanLogHandler{service: service, validator: validator}
}

// eventIDFromPath parses the {event_id} path parameter.
func eventIDFromPath(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	return id, nil
}

// RecordScan handles POST /events/{event_id}/scans.
//
// RecordScan godoc
//
//	@Summary		Record a scan
//	@Description	Resolves qr_code to a ticket scoped to this event and records a scan against workflow_step_id, enforcing the step's allows_multiple rule, ticket-type entitlement, and ticket status. Requires the check_in permission.
//	@Tags			scans
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string				true	"Event UUID"
//	@Param			body		body		scans.RecordScanInput	true	"Scan payload"
//	@Success		201			{object}	scans.ScanLog
//	@Failure		400			{object}	object	"Invalid request body, workflow step not on this event, or ticket type not entitled to this step"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing check_in permission"
//	@Failure		404			{object}	object	"Ticket or workflow step not found for this event"
//	@Failure		409			{object}	object	"Ticket invalidated, or workflow step already completed"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/scans [post]
func (h *ScanLogHandler) RecordScan(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body RecordScanInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.RecordScan(r.Context(), eventID, body)
	if err != nil {
		return nil, err
	}
	return response.Created(entity), nil
}

// List handles GET /events/{event_id}/scans with query parameters.
//
// List godoc
//
//	@Summary		List scan history
//	@Description	Returns a paginated list of scans for an event. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (ticket_id, workflow_step_id). Requires the check_in permission.
//	@Tags			scans
//	@Accept			json
//	@Produce		json
//	@Param			event_id			path		string	true	"Event UUID"
//	@Param			page				query		int		false	"Page number (1-based)"
//	@Param			size				query		int		false	"Page size (default 20, max 100)"
//	@Param			sort				query		string	false	"Sort: field,dir (e.g. sort=scanned_at,DESC)"
//	@Param			ticket_id			query		string	false	"Filter by ticket UUID (exact match)"
//	@Param			workflow_step_id	query		string	false	"Filter by workflow step UUID (exact match)"
//	@Success		200					{object}	common.PageResponse[scans.ScanLog]
//	@Failure		400					{object}	object	"Invalid event id or query"
//	@Failure		401					{object}	object	"Unauthenticated"
//	@Failure		403					{object}	object	"Missing check_in permission"
//	@Failure		500					{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/scans [get]
func (h *ScanLogHandler) List(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), ScanLogListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[ScanLog]
	result, err = h.service.ListScans(r.Context(), eventID, params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}
