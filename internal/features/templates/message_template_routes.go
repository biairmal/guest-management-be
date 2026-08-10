package templates

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitMessageTemplateRoutes registers message template routes on the given router.
func InitMessageTemplateRoutes(r *chi.Mux, messageTemplateH *MessageTemplateHandler) {
	r.Route("/api/v1/message-templates", func(r chi.Router) {
		r.Get("/", handler.Handle(messageTemplateH.List))
		r.Get("/{id}", handler.Handle(messageTemplateH.GetByID))
		r.Post("/", handler.Handle(messageTemplateH.Create))
		r.Put("/{id}", handler.Handle(messageTemplateH.Update))
		r.Delete("/{id}", handler.Handle(messageTemplateH.Delete))
	})
}
