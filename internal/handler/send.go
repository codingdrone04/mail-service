package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/codingdrone04/mail-service/internal/mail"
	"github.com/codingdrone04/mail-service/internal/validator"
)

type response struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message,omitempty"`
	Errors  []validator.ValidationError `json:"errors,omitempty"`
}

type SendHandler struct {
	mailer *mail.Mailer
}

func NewSendHandler(mailer *mail.Mailer) *SendHandler {
	return &SendHandler{mailer: mailer}
}

func (h *SendHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if ct := r.Header.Get("Content-Type"); ct != "application/json" {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		json.NewEncoder(w).Encode(response{
			Success: false,
			Message: "Content-Type must be application/json",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

	var form validator.ContactForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response{
			Success: false,
			Message: "Invalid JSON body",
		})
		return
	}

	validator.Sanitize(&form)

	if errs := validator.Validate(&form); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(response{
			Success: false,
			Message: "Validation failed",
			Errors:  errs,
		})
		return
	}

	if err := h.mailer.Send(form); err != nil {
		slog.Error("failed to send email",
			"error", err,
			"from_email", form.Email,
			"from_name", form.Name,
		)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response{
			Success: false,
			Message: "Failed to send message",
		})
		return
	}

	slog.Info("email sent successfully",
		"from_email", form.Email,
		"from_name", form.Name,
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response{
		Success: true,
		Message: "Message sent successfully",
	})
}
