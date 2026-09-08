package app

import (
	"context"
	"encoding/json"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/queue"
	"github.com/biairmal/guest-management-be/internal/features/guests"
)

// invitationTopic is the queue topic InvitationMessage is published to.
const invitationTopic = "guests.invitation"

// queueInvitationPublisher is the guests.InvitationPublisher implementation:
// a thin adapter over go-sdk's queue.Publisher (noop/logging/kafka, selected
// by internal/config.Config.Queue). This is the only file that imports
// go-sdk/queue for this purpose — guest_service.go depends only on the
// guests.InvitationPublisher interface, so swapping the backend later is a
// config change plus this file's wiring in service.go, nothing in guests.
// Mirrors cryptoPIIEncryptor's adapter-in-app pattern (pii_encryptor.go).
type queueInvitationPublisher struct {
	publisher queue.Publisher
}

// newQueueInvitationPublisher builds the guests.InvitationPublisher from pub.
func newQueueInvitationPublisher(pub queue.Publisher) guests.InvitationPublisher {
	return &queueInvitationPublisher{publisher: pub}
}

// PublishInvitation JSON-encodes msg and publishes it to invitationTopic,
// keyed by GuestID so backends that honor ordering (Kafka) keep one guest's
// messages in order.
func (p *queueInvitationPublisher) PublishInvitation(ctx context.Context, msg guests.InvitationMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("invitation publisher: marshal message")
	}
	return p.publisher.Publish(ctx, invitationTopic, body, queue.WithKey([]byte(msg.GuestID)))
}
