package guests

import (
	"context"

	"github.com/biairmal/go-sdk/lib/logger"
)

// InvitationMessage is what GuestService.SendInvitation hands to
// InvitationPublisher once a guest's invitation_token is generated. The
// consumer that actually sends an email/WhatsApp message from this (using
// the message_templates feature's invitation template) does not exist yet —
// out of scope for this phase.
type InvitationMessage struct {
	GuestID         string `json:"guest_id"`
	EventID         string `json:"event_id"`
	InvitationToken string `json:"invitation_token"`
}

// InvitationPublisher abstracts how an invitation "send" is dispatched to a
// background worker/consumer. go-sdk has no Kafka/queue package yet, so this
// is an app-local interface for now; internal/app wires LoggingInvitationPublisher
// this phase and can swap in a real Kafka-backed publisher later (once
// go-sdk ships one) by changing only that wiring — guests code doesn't
// change.
//
// No //go:generate mock is declared: its method signature references
// InvitationMessage (a guests-package type), so a generated mock would
// import "guests" and — mirroring PIIEncryptor's own note — cause an import
// cycle if consumed from this package's own tests. LoggingInvitationPublisher
// below has nothing meaningful to assert on (it only logs), so tests use it
// directly rather than a mock, per the "real no-op is fine where there's
// nothing to stub" carve-out in AGENTS.md.
type InvitationPublisher interface {
	// PublishInvitation dispatches msg. Called best-effort by
	// GuestService.SendInvitation: a failure is logged but does not fail the
	// invitation-send call, since the guest row/token are already correctly
	// persisted by that point.
	PublishInvitation(ctx context.Context, msg InvitationMessage) error
}

// loggingInvitationPublisher is the only InvitationPublisher implementation
// this phase: it just logs the message. See InvitationPublisher's doc for
// the swap-in-a-real-publisher-later plan.
type loggingInvitationPublisher struct {
	logger logger.Logger
}

// NewLoggingInvitationPublisher returns an InvitationPublisher that logs
// every invitation instead of actually dispatching it anywhere.
func NewLoggingInvitationPublisher(log logger.Logger) InvitationPublisher {
	return &loggingInvitationPublisher{logger: log}
}

// PublishInvitation logs msg and always returns nil.
func (p *loggingInvitationPublisher) PublishInvitation(ctx context.Context, msg InvitationMessage) error {
	p.logger.InfoWithContext(ctx, "invitation publish (logging stub, no real delivery)",
		logger.F("guest_id", msg.GuestID), logger.F("event_id", msg.EventID))
	return nil
}
