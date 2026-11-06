package config

import (
	"errors"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func files(m map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if v, ok := m[p]; ok {
			return []byte(v), nil
		}
		return nil, errors.New("no such file")
	}
}

func base() map[string]string {
	return map[string]string{
		"CORS_ALLOWED_ORIGINS": "https://portfolio.example.org",
		"SMTP_HOST":            "smtp.example.com",
		"SMTP_USER":            "u",
		"SMTP_PASSWORD":        "p",
		"MAIL_FROM":            "contact@example.org",
		"MAIL_TO":              "me@example.com",
	}
}

const monSite = `{"sites":[{"id":"mon-site","name":"Mon Site","origins":["https://www.example.com"],"to":["contact@example.com"],"lang":"fr","require_contact":"phone","daily_limit":50}]}`

func TestDefaultSiteUnchanged(t *testing.T) {
	cfg, err := Parse(env(base()), files(nil))
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Default
	if len(d.To) != 1 || d.To[0] != "me@example.com" || d.SubjectPrefix != "[Portfolio Contact]" || d.RequireContact != "email" {
		t.Fatalf("default site changed: %+v", d)
	}
	if len(cfg.Sites) != 0 || d.Location == nil {
		t.Fatalf("unexpected: %+v", cfg)
	}
}

func TestSitesFile(t *testing.T) {
	m := base()
	m["SITES_FILE"] = "/app/config/sites.json"
	cfg, err := Parse(env(m), files(map[string]string{"/app/config/sites.json": monSite}))
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Sites["mon-site"]
	if s.To[0] != "contact@example.com" || s.RequireContact != "phone" || s.SubjectPrefix != "[mon-site]" || s.Timezone != "Europe/Paris" || s.Location == nil {
		t.Fatalf("bad site: %+v", s)
	}
	if !s.OriginAllowed("https://www.example.com") || s.OriginAllowed("https://portfolio.example.org") {
		t.Fatal("origin check wrong")
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("CORS origins should be the union, got %v", cfg.AllowedOrigins)
	}
}

func TestSitesFileErrors(t *testing.T) {
	cases := map[string]string{
		"unknown key":    `{"sites":[{"id":"a","origins":["https://a.fr"],"to":["a@a.fr"],"recipient":"x"}]}`,
		"bad id":         `{"sites":[{"id":"Bad Id","origins":["https://a.fr"],"to":["a@a.fr"]}]}`,
		"duplicate":      `{"sites":[{"id":"a","origins":["https://a.fr"],"to":["a@a.fr"]},{"id":"a","origins":["https://a.fr"],"to":["a@a.fr"]}]}`,
		"no recipient":   `{"sites":[{"id":"a","origins":["https://a.fr"]}]}`,
		"origin path":    `{"sites":[{"id":"a","origins":["https://a.fr/"],"to":["a@a.fr"]}]}`,
		"bad require":    `{"sites":[{"id":"a","origins":["https://a.fr"],"to":["a@a.fr"],"require_contact":"fax"}]}`,
		"bad timezone":   `{"sites":[{"id":"a","origins":["https://a.fr"],"to":["a@a.fr"],"timezone":"Mars/Olympus"}]}`,
		"multiline name": `{"sites":[{"id":"a","name":"A\nBcc: x","origins":["https://a.fr"],"to":["a@a.fr"]}]}`,
	}
	for name, raw := range cases {
		m := base()
		m["SITES_FILE"] = "s.json"
		if _, err := Parse(env(m), files(map[string]string{"s.json": raw})); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	m := base()
	delete(m, "MAIL_TO")
	if _, err := Parse(env(m), files(nil)); err == nil || !strings.Contains(err.Error(), "MAIL_TO") {
		t.Fatalf("missing MAIL_TO should be reported, got %v", err)
	}
}
