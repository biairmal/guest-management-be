package tickettypetemplate

import "encoding/json"

// CreateTicketTypeTemplateInput is the input for creating a ticket type
// template under an event category. category_id is taken from the URL, not
// part of this input. rules is an opaque JSON document, passed through
// unvalidated; it defaults to an empty object when omitted, matching the
// column default.
//
// swagger:model CreateTicketTypeTemplateInput
type CreateTicketTypeTemplateInput struct {
	Name  string          `json:"name"            validate:"required"`
	Rules json.RawMessage `json:"rules,omitempty"`
}

// UpdateTicketTypeTemplateInput is the input for partially updating a ticket
// type template's name/rules. category_id is immutable and not part of this
// input.
//
// swagger:model UpdateTicketTypeTemplateInput
type UpdateTicketTypeTemplateInput struct {
	Name  *string         `json:"name,omitempty"  validate:"omitempty,min=1"`
	Rules json.RawMessage `json:"rules,omitempty"`
}
