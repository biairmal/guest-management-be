package templates

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
)

// assertErrorzCode fails unless err carries the wanted errorz code (or is nil when want == "").
func assertErrorzCode(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var e *errorz.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errorz.Error, got %T: %v", err, err)
	}
	if e.Code != want {
		t.Errorf("code = %q, want %q", e.Code, want)
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func ptrString(s string) *string { return &s }

func TestMessageTemplateService_Create(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()

	tests := []struct {
		name    string
		in      CreateInput
		expects bool // whether repo.Create is reached (invariant failures short-circuit)
		repoErr error
		wantErr string
	}{
		{
			name:    "app source rejects tenant_id",
			in:      CreateInput{Source: SourceApp, TenantID: ptrUUID(tenantID), Name: "x", Channel: ChannelWhatsapp, Body: "b"},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name:    "app source rejects event_id",
			in:      CreateInput{Source: SourceApp, EventID: ptrUUID(eventID), Name: "x", Channel: ChannelWhatsapp, Body: "b"},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name:    "tenant source requires tenant_id",
			in:      CreateInput{Source: SourceTenant, Name: "x", Channel: ChannelWhatsapp, Body: "b"},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name: "tenant source rejects event_id",
			in: CreateInput{
				Source: SourceTenant, TenantID: ptrUUID(tenantID), EventID: ptrUUID(eventID),
				Name: "x", Channel: ChannelWhatsapp, Body: "b",
			},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name: "event source requires tenant_id and event_id",
			in: CreateInput{
				Source: SourceEvent, TenantID: ptrUUID(tenantID), Name: "x", Channel: ChannelWhatsapp, Body: "b",
			},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name:    "email channel requires subject",
			in:      CreateInput{Source: SourceApp, Name: "x", Channel: ChannelEmail, Body: "b"},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name: "whatsapp channel rejects subject",
			in: CreateInput{
				Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Subject: ptrString("Hi"), Body: "b",
			},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name:    "already exists maps to 409",
			in:      CreateInput{Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Body: "b"},
			expects: true,
			repoErr: repository.ErrAlreadyExists,
			wantErr: errorz.CodeConflict,
		},
		{
			name:    "invalid entity maps to 422",
			in:      CreateInput{Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Body: "b"},
			expects: true,
			repoErr: repository.ErrInvalidEntity,
			wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name:    "unexpected repo error maps to 500",
			in:      CreateInput{Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Body: "b"},
			expects: true,
			repoErr: errors.New("boom"),
			wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path event scope with email",
			in: CreateInput{
				Source: SourceEvent, TenantID: ptrUUID(tenantID), EventID: ptrUUID(eventID),
				Name: "Invitation", Channel: ChannelEmail, Subject: ptrString("You're invited"), Body: "b",
			},
			expects: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[MessageTemplate, uuid.UUID](ctrl)
			if tt.expects {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			}

			svc := NewMessageTemplateService(logger.NewNoOp(), repo)
			got, err := svc.Create(context.Background(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got == nil {
				t.Fatal("expected non-nil entity on success")
			}
		})
	}
}

func TestMessageTemplateService_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		repoRes *MessageTemplate
		repoErr error
		wantErr string
	}{
		{name: "found", repoRes: &MessageTemplate{Name: "x"}},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[MessageTemplate, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewMessageTemplateService(logger.NewNoOp(), repo)
			_, err := svc.GetByID(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestMessageTemplateService_Update(t *testing.T) {
	tenantID := uuid.New()
	tests := []struct {
		name       string
		in         UpdateInput
		getRes     *MessageTemplate
		getErr     error
		expectsSet bool
		updateErr  error
		wantErr    string
	}{
		{
			name:    "get not found maps to 404",
			getErr:  repository.ErrNotFound,
			wantErr: errorz.CodeNotFound,
		},
		{
			name: "resulting record violates invariant",
			in:   UpdateInput{Source: ptrString(SourceTenant)},
			getRes: &MessageTemplate{
				Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Body: "b",
			},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name: "update conflict maps to 409",
			in:   UpdateInput{Name: ptrString("y")},
			getRes: &MessageTemplate{
				Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Body: "b",
			},
			expectsSet: true,
			updateErr:  repository.ErrAlreadyExists,
			wantErr:    errorz.CodeConflict,
		},
		{
			name: "happy path partial update",
			in:   UpdateInput{TenantID: ptrUUID(tenantID), Source: ptrString(SourceTenant)},
			getRes: &MessageTemplate{
				Source: SourceApp, Name: "x", Channel: ChannelWhatsapp, Body: "b",
			},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[MessageTemplate, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewMessageTemplateService(logger.NewNoOp(), repo)
			got, err := svc.Update(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got == nil {
				t.Fatal("expected non-nil entity on success")
			}
		})
	}
}

func TestMessageTemplateService_Delete(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		wantErr string
	}{
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{name: "happy path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[MessageTemplate, uuid.UUID](ctrl)
			repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewMessageTemplateService(logger.NewNoOp(), repo)
			err := svc.Delete(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestMessageTemplateService_List(t *testing.T) {
	tests := []struct {
		name     string
		repoErr  error
		wantErr  string
		wantSize int
	}{
		{name: "repo error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{name: "happy path", wantSize: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[MessageTemplate, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return([]*MessageTemplate{{Name: "x"}}, int64(1), tt.repoErr)

			svc := NewMessageTemplateService(logger.NewNoOp(), repo)
			params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
			if err != nil {
				t.Fatalf("ParseListParams() error = %v", err)
			}
			got, err := svc.List(context.Background(), params)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", got.Size, tt.wantSize)
			}
		})
	}
}
