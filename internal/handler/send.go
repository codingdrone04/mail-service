package handler

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/mail"
	"github.com/codingdrone04/mail-service/internal/middleware"
	"github.com/codingdrone04/mail-service/internal/validator"
)

type response struct {
	Success bool                        `json:"success"`
	Message string                      `json:"message,omitempty"`
	Errors  []validator.ValidationError `json:"errors,omitempty"`
}

type SendHandler struct {
	sender      mail.Sender
	defaultSite config.Site
	sites       map[string]config.Site
	limiter     *middleware.Limiter
	perMinute   int // default per-IP limit when a site sets none
}

func NewSendHandler(sender mail.Sender, cfg config.Config, limiter *middleware.Limiter) *SendHandler {
	return &SendHandler{sender: sender, defaultSite: cfg.Default, sites: cfg.Sites, limiter: limiter, perMinute: cfg.RateLimitPerMin}
}

func reply(w http.ResponseWriter, status int, r response) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(r)
}

func siteLog(s config.Site) string {
	if s.ID == "" {
		return "default"
	}
	return s.ID
}

// Handle serves POST /send (default site) and POST /send/{site}.
func (h *SendHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	site := h.defaultSite
	if id := r.PathValue("site"); id != "" {
		s, ok := h.sites[id]
		if !ok {
			reply(w, http.StatusNotFound, response{Success: false, Message: "Unknown site"})
			return
		}
		site = s
	}
	log := slog.With("site", siteLog(site))

	// CORS only protects browsers' reads; also refuse a browser post coming from
	// another site's origin, so each recipient only receives its own site's forms.
	if origin := r.Header.Get("Origin"); origin != "" && !site.OriginAllowed(origin) {
		log.Warn("origin refused", "origin", origin)
		reply(w, http.StatusForbidden, response{Success: false, Message: "Origin not allowed"})
		return
	}

	// Per-IP limit, counted per site so one project's traffic never blocks another's.
	perMinute := site.RateLimitPerMin
	if perMinute == 0 {
		perMinute = h.perMinute
	}
	if !h.limiter.Allow("ip|"+site.ID+"|"+middleware.ClientIP(r), perMinute, time.Minute) {
		w.Header().Set("Retry-After", "60")
		reply(w, http.StatusTooManyRequests, response{Success: false, Message: "Too many requests, please try again later"})
		return
	}

	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		reply(w, http.StatusUnsupportedMediaType, response{Success: false, Message: "Content-Type must be application/json"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB is plenty for a contact form

	var form validator.ContactForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		reply(w, http.StatusBadRequest, response{Success: false, Message: "Invalid JSON body"})
		return
	}

	validator.Sanitize(&form)

	// Honeypot: if the hidden "company" field is filled, it's a bot. Fake a success.
	if form.Company != "" {
		log.Info("honeypot triggered")
		reply(w, http.StatusOK, response{Success: true, Message: "Message sent successfully"})
		return
	}

	if errs := validator.Validate(&form, validator.Rules{RequireContact: site.RequireContact}); len(errs) > 0 {
		reply(w, http.StatusUnprocessableEntity, response{Success: false, Message: "Validation failed", Errors: errs})
		return
	}

	// Whole-site daily cap: a safety net if a bot gets past the honeypot.
	if !h.limiter.Allow("day|"+site.ID, site.DailyLimit, 24*time.Hour) {
		log.Warn("daily limit reached", "limit", site.DailyLimit)
		reply(w, http.StatusTooManyRequests, response{Success: false, Message: "Daily limit reached, please try again tomorrow"})
		return
	}

	start := time.Now()
	if err := h.sender.Send(form, site); err != nil {
		log.Error("failed to send email", "error", err, "duration_ms", time.Since(start).Milliseconds())
		reply(w, http.StatusBadGateway, response{Success: false, Message: "Failed to send message"})
		return
	}

	// No personal data in the logs: only the site and the timing.
	log.Info("email sent", "duration_ms", time.Since(start).Milliseconds(), "fields", len(form.Fields))
	reply(w, http.StatusOK, response{Success: true, Message: "Message sent successfully"})
}
