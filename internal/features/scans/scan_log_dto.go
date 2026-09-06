package scans

import "github.com/google/uuid"

// RecordScanInput is the input for POST .../scans. The client identifies the
// ticket by qr_code — the QR value the guest actually presents — never a
// resolved ticket_id; the server resolves qr_code to a Ticket internally,
// scoped to the event_id in the URL.
//
// swagger:model RecordScanInput
type RecordScanInput struct {
	QRCode         string    `json:"qr_code"          validate:"required"`
	WorkflowStepID uuid.UUID `json:"workflow_step_id" validate:"required"`
}
