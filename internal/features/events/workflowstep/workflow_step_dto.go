package workflowstep

import "github.com/google/uuid"

// SyncWorkflowStepInput is one entry in a Service.Sync request: an existing
// step to update when ID is set, or a new step to create when ID is omitted.
//
// Not an HTTP payload since B16 — the event's setup save (events/event's
// ReplaceSetup) builds these from its own request body, so there is no
// swagger:model and no validate tags; the setup input carries those. The
// per-step CreateWorkflowStepInput/UpdateWorkflowStepInput went with the write
// routes they served.
type SyncWorkflowStepInput struct {
	ID             *uuid.UUID
	Name           string
	OrderIndex     int
	AllowsMultiple bool
}
