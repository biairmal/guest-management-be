package guests

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	mockguests "github.com/biairmal/guest-management-be/mocks/guests"
)

// newTestService wires a guestServiceImpl for tests. encryptor may be nil for
// tests that never touch email/phone filters (List's rewriteEncryptedFilters
// is the only caller). publisher defaults to the real logging implementation
// (nothing to assert on it — see invitation_publisher.go).
func newTestService(
	repo repository.Repository[Guest, uuid.UUID],
	ticketRepo repository.Repository[Ticket, uuid.UUID],
	eventRepo repository.ReadRepository[event.Event, uuid.UUID],
	ticketTypeRepo repository.ReadRepository[tickets.TicketType, uuid.UUID],
	encryptor PIIEncryptor,
) GuestService {
	return NewGuestService(
		logger.NewNoOp(), repo, ticketRepo, eventRepo, ticketTypeRepo, encryptor,
		NewLoggingInvitationPublisher(logger.NewNoOp()),
	)
}

func TestGuestService_Create(t *testing.T) {
	eventID := uuid.New()

	t.Run("ticket type not found maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketTypeRepo := mockrepository.NewMockRepository[tickets.TicketType, uuid.UUID](ctrl)
		ticketTypeID := uuid.New()
		ticketTypeRepo.EXPECT().GetByID(gomock.Any(), ticketTypeID).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil, ticketTypeRepo, nil)

		_, err := svc.Create(context.Background(), eventID, CreateGuestInput{
			Name: "Alice", Email: "alice@example.com", TicketTypeID: &ticketTypeID,
		})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("ticket type from a different event maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketTypeRepo := mockrepository.NewMockRepository[tickets.TicketType, uuid.UUID](ctrl)
		ticketTypeID := uuid.New()
		ticketTypeRepo.EXPECT().GetByID(gomock.Any(), ticketTypeID).
			Return(&tickets.TicketType{ID: ticketTypeID, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, nil, ticketTypeRepo, nil)

		_, err := svc.Create(context.Background(), eventID, CreateGuestInput{
			Name: "Alice", Email: "alice@example.com", TicketTypeID: &ticketTypeID,
		})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("invalid entity maps to 422", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrInvalidEntity)
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.Create(context.Background(), eventID, CreateGuestInput{Name: "Alice", Email: "alice@example.com"})
		assertErrorzCode(t, err, errorz.CodeUnprocessableEntity)
	})

	t.Run("unexpected create error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.Create(context.Background(), eventID, CreateGuestInput{Name: "Alice", Email: "alice@example.com"})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path starts rsvp_status at none", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		got, err := svc.Create(context.Background(), eventID, CreateGuestInput{Name: "Alice", Email: "alice@example.com"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got.RsvpStatus != RsvpStatusNone {
			t.Errorf("RsvpStatus = %q, want %q", got.RsvpStatus, RsvpStatusNone)
		}
		if got.EventID != eventID {
			t.Errorf("EventID = %v, want %v", got.EventID, eventID)
		}
	})
}

func TestGuestService_GetByID(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("found for different event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("no ticket issued leaves Ticket nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID}, nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		got, err := svc.GetByID(context.Background(), eventID, id)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Ticket != nil {
			t.Errorf("Ticket = %+v, want nil", got.Ticket)
		}
	})

	t.Run("ticket lookup failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[Ticket, uuid.UUID](ctrl)
		ticketID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID, TicketID: &ticketID}, nil)
		ticketRepo.EXPECT().GetByID(gomock.Any(), ticketID).Return(nil, errors.New("boom"))
		svc := newTestService(repo, ticketRepo, nil, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path populates Ticket", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[Ticket, uuid.UUID](ctrl)
		ticketID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID, TicketID: &ticketID}, nil)
		ticketRepo.EXPECT().GetByID(gomock.Any(), ticketID).Return(&Ticket{ID: ticketID, QRCode: "qr"}, nil)
		svc := newTestService(repo, ticketRepo, nil, nil, nil)

		got, err := svc.GetByID(context.Background(), eventID, id)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Ticket == nil || got.Ticket.QRCode != "qr" {
			t.Errorf("Ticket = %+v, want QRCode=qr", got.Ticket)
		}
	})
}

func TestGuestService_Update(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("get not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.Update(context.Background(), eventID, id, UpdateGuestInput{})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("ticket type cross-event maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketTypeRepo := mockrepository.NewMockRepository[tickets.TicketType, uuid.UUID](ctrl)
		ticketTypeID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID}, nil)
		ticketTypeRepo.EXPECT().GetByID(gomock.Any(), ticketTypeID).
			Return(&tickets.TicketType{ID: ticketTypeID, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, nil, ticketTypeRepo, nil)

		_, err := svc.Update(context.Background(), eventID, id, UpdateGuestInput{TicketTypeID: &ticketTypeID})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("happy path partial update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID, Name: "Alice"}, nil)
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		got, err := svc.Update(context.Background(), eventID, id, UpdateGuestInput{Name: ptrString("Bob")})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if got.Name != "Bob" {
			t.Errorf("Name = %q, want Bob", got.Name)
		}
	})
}

func TestGuestService_Delete(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil, nil, nil)

		err := svc.Delete(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID}, nil)
		repo.EXPECT().Delete(gomock.Any(), id).Return(nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		if err := svc.Delete(context.Background(), eventID, id); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
	})
}

func TestGuestService_List(t *testing.T) {
	eventID := uuid.New()

	t.Run("email filter with like operator is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		svc := newTestService(repo, nil, nil, nil, nil)

		params := &query.ListParams{Filters: map[string]query.FilterValue{
			"email": {Value: "alice@example.com", Operator: repository.FilterOperatorLike},
		}}
		_, err := svc.List(context.Background(), eventID, params)
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("email filter is rewritten to a blind-index condition on email_hash", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		encryptor := mockguests.NewMockPIIEncryptor(ctrl)
		encryptor.EXPECT().BlindIndex("alice@example.com").Return("hashed-email", nil)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*Guest, int64, error) {
				found := false
				for _, c := range opts.Filter.Conditions {
					if c.Field == "email_hash" && c.Value == "hashed-email" && c.Operator == repository.FilterOperatorEq {
						found = true
					}
					if c.Field == "email" {
						t.Error("expected the plaintext email filter to be removed")
					}
				}
				if !found {
					t.Error("expected an email_hash eq condition")
				}
				return nil, 0, nil
			})
		svc := newTestService(repo, nil, nil, nil, encryptor)

		params := &query.ListParams{Filters: map[string]query.FilterValue{
			"email": {Value: " Alice@Example.com ", Operator: repository.FilterOperatorEq},
		}}
		if _, err := svc.List(context.Background(), eventID, params); err != nil {
			t.Fatalf("List() error = %v", err)
		}
	})

	t.Run("always injects event_id filter regardless of query params", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*Guest, int64, error) {
				found := false
				for _, c := range opts.Filter.Conditions {
					if c.Field == "event_id" && c.Value == eventID {
						found = true
					}
				}
				if !found {
					t.Error("expected event_id filter condition to be present and match eventID")
				}
				return []*Guest{{ID: uuid.New(), EventID: eventID}}, int64(1), nil
			})
		svc := newTestService(repo, nil, nil, nil, nil)

		params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		got, err := svc.List(context.Background(), eventID, params)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if got.Size != 20 {
			t.Errorf("Size = %d, want 20", got.Size)
		}
	})
}

func TestGuestService_SendInvitation(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("no ticket type assigned maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID}, nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.SendInvitation(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("rsvp_required event does not issue a ticket yet", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[event.Event, uuid.UUID](ctrl)
		ticketTypeID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID, TicketTypeID: &ticketTypeID}, nil)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&event.Event{ID: eventID, RsvpRequired: true}, nil)
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, g *Guest) error {
				if g.TicketID != nil {
					t.Error("expected no ticket to be issued when rsvp_required is true")
				}
				if g.RsvpStatus != RsvpStatusInvited {
					t.Errorf("RsvpStatus = %q, want %q", g.RsvpStatus, RsvpStatusInvited)
				}
				return nil
			})
		svc := newTestService(repo, nil, eventRepo, nil, nil)

		out, err := svc.SendInvitation(context.Background(), eventID, id)
		if err != nil {
			t.Fatalf("SendInvitation() error = %v", err)
		}
		if out.InvitationToken == "" {
			t.Error("expected a non-empty invitation token")
		}
	})

	t.Run("rsvp not required issues a ticket immediately", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[Ticket, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[event.Event, uuid.UUID](ctrl)
		ticketTypeID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID, TicketTypeID: &ticketTypeID}, nil)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&event.Event{ID: eventID, RsvpRequired: false}, nil)
		ticketRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, g *Guest) error {
				if g.TicketID == nil {
					t.Error("expected a ticket to be issued when rsvp_required is false")
				}
				return nil
			})
		svc := newTestService(repo, ticketRepo, eventRepo, nil, nil)

		if _, err := svc.SendInvitation(context.Background(), eventID, id); err != nil {
			t.Fatalf("SendInvitation() error = %v", err)
		}
	})

	t.Run("event lookup failure fails closed (no ticket issued)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[event.Event, uuid.UUID](ctrl)
		ticketTypeID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&Guest{ID: id, EventID: eventID, TicketTypeID: &ticketTypeID}, nil)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(nil, errors.New("boom"))
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, g *Guest) error {
				if g.TicketID != nil {
					t.Error("expected no ticket to be issued when the event lookup fails")
				}
				return nil
			})
		svc := newTestService(repo, nil, eventRepo, nil, nil)

		if _, err := svc.SendInvitation(context.Background(), eventID, id); err != nil {
			t.Fatalf("SendInvitation() error = %v", err)
		}
	})
}

func TestGuestService_RSVP(t *testing.T) {
	t.Run("unknown token maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		svc := newTestService(repo, nil, nil, nil, nil)

		_, err := svc.RSVP(context.Background(), "unknown-token", RsvpStatusConfirmed)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("confirm issues a ticket when none exists yet", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[Ticket, uuid.UUID](ctrl)
		id, ticketTypeID := uuid.New(), uuid.New()
		guest := &Guest{ID: id, TicketTypeID: &ticketTypeID}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*Guest{guest}, int64(1), nil)
		ticketRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, g *Guest) error {
				if g.TicketID == nil {
					t.Error("expected a ticket to be issued on confirm")
				}
				if g.RsvpStatus != RsvpStatusConfirmed {
					t.Errorf("RsvpStatus = %q, want %q", g.RsvpStatus, RsvpStatusConfirmed)
				}
				return nil
			})
		svc := newTestService(repo, ticketRepo, nil, nil, nil)

		if _, err := svc.RSVP(context.Background(), "tok", RsvpStatusConfirmed); err != nil {
			t.Fatalf("RSVP() error = %v", err)
		}
	})

	t.Run("confirm is idempotent when a ticket was already issued", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[Ticket, uuid.UUID](ctrl)
		id, ticketID := uuid.New(), uuid.New()
		guest := &Guest{ID: id, TicketID: &ticketID}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*Guest{guest}, int64(1), nil)
		// ticketRepo.Create is deliberately not expected — issueTicketIfNeeded must no-op.
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(nil)
		svc := newTestService(repo, ticketRepo, nil, nil, nil)

		if _, err := svc.RSVP(context.Background(), "tok", RsvpStatusConfirmed); err != nil {
			t.Fatalf("RSVP() error = %v", err)
		}
	})

	t.Run("decline does not touch an already-issued ticket", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Guest, uuid.UUID](ctrl)
		id, ticketID := uuid.New(), uuid.New()
		guest := &Guest{ID: id, TicketID: &ticketID}
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*Guest{guest}, int64(1), nil)
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, g *Guest) error {
				if g.TicketID == nil || *g.TicketID != ticketID {
					t.Error("expected the existing ticket to stand on decline")
				}
				if g.RsvpStatus != RsvpStatusDeclined {
					t.Errorf("RsvpStatus = %q, want %q", g.RsvpStatus, RsvpStatusDeclined)
				}
				return nil
			})
		svc := newTestService(repo, nil, nil, nil, nil)

		if _, err := svc.RSVP(context.Background(), "tok", RsvpStatusDeclined); err != nil {
			t.Fatalf("RSVP() error = %v", err)
		}
	})
}
