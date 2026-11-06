package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/middleware"
	"github.com/codingdrone04/mail-service/internal/validator"
)

type fakeSender struct {
	sent  []config.Site
	forms []validator.ContactForm
}

func (f *fakeSender) Send(form validator.ContactForm, s config.Site) error {
	f.sent = append(f.sent, s)
	f.forms = append(f.forms, form)
	return nil
}

func setup() (*fakeSender, http.Handler) {
	cfg := config.Config{
		RateLimitPerMin: 100,
		Default:         config.Site{Origins: []string{"https://portfolio.example.org"}, To: []string{"me@x.fr"}, RequireContact: "email"},
		Sites: map[string]config.Site{
			"mon-site": {ID: "mon-site", Origins: []string{"https://www.example.com"}, To: []string{"contact@example.com"}, RequireContact: "phone", Lang: "fr", DailyLimit: 3},
		},
	}
	f := &fakeSender{}
	h := NewSendHandler(f, cfg, middleware.NewLimiter())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /send", h.Handle)
	mux.HandleFunc("POST /send/{site}", h.Handle)
	return f, mux
}

func post(h http.Handler, path, origin, ip, body string) int {
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("CF-Connecting-IP", ip)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

const siteForm = `{"name":"Jean","phone":"06 12 34 56 78","fields":{"Ville":"Anglet","Type de travaux":"Peinture"}}`

func TestRouting(t *testing.T) {
	f, h := setup()
	if c := post(h, "/send/mon-site", "https://www.example.com", "1.1.1.1", siteForm); c != 200 {
		t.Fatalf("site post: %d", c)
	}
	if len(f.sent) != 1 || f.sent[0].To[0] != "contact@example.com" || f.forms[0].Fields[0].Label != "Ville" {
		t.Fatalf("wrong delivery: %+v %+v", f.sent, f.forms)
	}
	if c := post(h, "/send/mon-site", "https://portfolio.example.org", "1.1.1.1", siteForm); c != 403 {
		t.Fatalf("cross-site origin should be 403, got %d", c)
	}
	if c := post(h, "/send/inconnu", "", "1.1.1.1", siteForm); c != 404 {
		t.Fatalf("unknown site should be 404, got %d", c)
	}
	if c := post(h, "/send", "https://portfolio.example.org", "1.1.1.1", siteForm); c != 422 {
		t.Fatalf("default site still requires email, got %d", c)
	}
	if c := post(h, "/send", "https://portfolio.example.org", "1.1.1.1", `{"name":"A","email":"a@b.co","message":"m"}`); c != 200 {
		t.Fatalf("default site post: %d", c)
	}
	n := len(f.sent)
	if c := post(h, "/send/mon-site", "", "1.1.1.1", `{"name":"Bot","phone":"0600000000","message":"x","company":"spam"}`); c != 200 || len(f.sent) != n {
		t.Fatalf("honeypot must fake success without sending: %d", c)
	}
}

func TestDailyLimit(t *testing.T) {
	f, h := setup()
	for i, ip := range []string{"1.0.0.1", "1.0.0.2", "1.0.0.3"} {
		if c := post(h, "/send/mon-site", "", ip, siteForm); c != 200 {
			t.Fatalf("send %d: %d", i, c)
		}
	}
	if c := post(h, "/send/mon-site", "", "1.0.0.4", siteForm); c != 429 {
		t.Fatalf("daily limit should give 429, got %d", c)
	}
	if c := post(h, "/send", "", "1.0.0.4", `{"name":"A","email":"a@b.co","message":"m"}`); c != 200 {
		t.Fatalf("another site must not be affected, got %d", c)
	}
	if len(f.sent) != 4 {
		t.Fatalf("sent %d", len(f.sent))
	}
}
