package guests

import "context"

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
// background worker/consumer. internal/app wires the one concrete
// implementation — a thin adapter over go-sdk's queue.Publisher, backend
// (noop/logging/kafka) selected by internal/config.Config.Queue — mirroring
// how pii_encryptor.go adapts go-sdk's crypto package for PIIEncryptor. This
// package only ever depends on the interface, so changing the backend later
// is a config + internal/app change, nothing here.
//
// No //go:generate mock is declared: its method signature references
// InvitationMessage (a guests-package type), so a generated mock would
// import "guests" — and guest_service__test.go (package guests) importing a
// mock package that itself imports guests is a real import cycle (unlike
// PIIEncryptor's mock, whose methods are all plain strings/error, so its
// mock never needs to import guests). Tests that need to assert on a publish
// pass a hand-written stub implementing this interface instead.
type InvitationPublisher interface {
	// PublishInvitation dispatches msg. Called best-effort by
	// GuestService.SendInvitation: a failure is logged but does not fail the
	// invitation-send call, since the guest row/token are already correctly
	// persisted by that point.
	PublishInvitation(ctx context.Context, msg InvitationMessage) error
}
