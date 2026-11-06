package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the Alpine image has no zoneinfo
)

// Site describes one project allowed to send through the service.
// The default site (ID "") is built from environment variables and keeps the
// historical behaviour of POST /send.
type Site struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`                  // shown in the email footer, e.g. "My Site"
	Origins         []string `json:"origins"`               // browser origins allowed to post
	To              []string `json:"to"`                    // recipients
	SubjectPrefix   string   `json:"subject_prefix"`        // e.g. "[My Site]"
	Lang            string   `json:"lang"`                  // "en" (default) or "fr"
	RequireContact  string   `json:"require_contact"`       // "email" (default), "phone" or "any"
	RateLimitPerMin int      `json:"rate_limit_per_minute"` // per IP, 0 = global default
	DailyLimit      int      `json:"daily_limit"`           // whole site, 0 = unlimited
	Timezone        string   `json:"timezone"`              // IANA name for the email date, default Europe/Paris

	Location *time.Location `json:"-"`
}

type Config struct {
	Port            string
	AllowedOrigins  []string // union of every site's origins, used by the CORS middleware
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPassword    string
	MailFrom        string
	SMTPTimeout     time.Duration
	RateLimitPerMin int
	Default         Site
	Sites           map[string]Site // extra sites, served on POST /send/{id}
}

var siteID = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// Load reads the environment (and SITES_FILE if set) and stops the process on error.
func Load() Config {
	cfg, err := Parse(os.Getenv, os.ReadFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	return cfg
}

// Parse builds the configuration from getenv and, when SITES_FILE is set, from
// the JSON file it points to (read through readFile).
func Parse(getenv func(string) string, readFile func(string) ([]byte, error)) (Config, error) {
	var missing []string
	req := func(key string) string {
		val := getenv(key)
		if val == "" {
			missing = append(missing, key)
		}
		return val
	}
	def := func(key, fallback string) string {
		if val := getenv(key); val != "" {
			return val
		}
		return fallback
	}

	cfg := Config{
		Port:            def("SERVER_PORT", "8080"),
		SMTPHost:        req("SMTP_HOST"),
		SMTPPort:        def("SMTP_PORT", "587"),
		SMTPUser:        req("SMTP_USER"),
		SMTPPassword:    req("SMTP_PASSWORD"),
		MailFrom:        req("MAIL_FROM"),
		SMTPTimeout:     time.Duration(atoiOr(getenv("SMTP_TIMEOUT_SECONDS"), 10)) * time.Second,
		RateLimitPerMin: atoiOr(getenv("RATE_LIMIT_PER_MINUTE"), 5),
		Sites:           map[string]Site{},
	}
	cfg.Default = Site{
		Name:           def("SITE_NAME", ""),
		Origins:        parseList(req("CORS_ALLOWED_ORIGINS")),
		To:             parseList(req("MAIL_TO")),
		SubjectPrefix:  def("SUBJECT_PREFIX", "[Portfolio Contact]"),
		Lang:           "en",
		RequireContact: "email",
		Timezone:       def("TIMEZONE", "Europe/Paris"),
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}
	if _, err := mail.ParseAddress(cfg.MailFrom); err != nil {
		return cfg, fmt.Errorf("MAIL_FROM is not a valid address: %v", err)
	}
	if err := validateSite(&cfg.Default); err != nil {
		return cfg, fmt.Errorf("default site: %w", err)
	}
	cfg.AllowedOrigins = append([]string{}, cfg.Default.Origins...)

	if path := getenv("SITES_FILE"); path != "" {
		raw, err := readFile(path)
		if err != nil {
			return cfg, fmt.Errorf("reading SITES_FILE: %w", err)
		}
		var file struct {
			Sites []Site `json:"sites"`
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields() // a typo in a key must fail loudly, not be ignored
		if err := dec.Decode(&file); err != nil {
			return cfg, fmt.Errorf("parsing %s: %w", path, err)
		}
		for _, s := range file.Sites {
			if !siteID.MatchString(s.ID) {
				return cfg, fmt.Errorf("site id %q: use 1-32 chars a-z, 0-9, -", s.ID)
			}
			if _, dup := cfg.Sites[s.ID]; dup {
				return cfg, fmt.Errorf("site id %q declared twice", s.ID)
			}
			if err := validateSite(&s); err != nil {
				return cfg, fmt.Errorf("site %q: %w", s.ID, err)
			}
			cfg.Sites[s.ID] = s
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, s.Origins...)
		}
	}
	return cfg, nil
}

// validateSite checks a site and fills its defaults.
func validateSite(s *Site) error {
	if len(s.Origins) == 0 {
		return fmt.Errorf("at least one origin is required")
	}
	for _, o := range s.Origins {
		if o == "*" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != "" {
			return fmt.Errorf("origin %q must look like https://example.com (no path, no trailing slash)", o)
		}
	}
	if len(s.To) == 0 {
		return fmt.Errorf("at least one recipient (to) is required")
	}
	for _, t := range s.To {
		if a, err := mail.ParseAddress(t); err != nil || a.Address != t {
			return fmt.Errorf("recipient %q is not a plain email address", t)
		}
	}
	if s.Lang == "" {
		s.Lang = "en"
	}
	if _, ok := labelsLangs[s.Lang]; !ok {
		return fmt.Errorf("lang must be one of en, fr")
	}
	if s.RequireContact == "" {
		s.RequireContact = "email"
	}
	switch s.RequireContact {
	case "email", "phone", "any":
	default:
		return fmt.Errorf("require_contact must be email, phone or any")
	}
	if s.SubjectPrefix == "" && s.ID != "" {
		s.SubjectPrefix = "[" + s.ID + "]"
	}
	if strings.ContainsAny(s.SubjectPrefix+s.Name, "\r\n") {
		return fmt.Errorf("name and subject_prefix must be single-line")
	}
	if s.Timezone == "" {
		s.Timezone = "Europe/Paris"
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return fmt.Errorf("unknown timezone %q", s.Timezone)
	}
	s.Location = loc
	if s.RateLimitPerMin < 0 || s.DailyLimit < 0 {
		return fmt.Errorf("limits cannot be negative")
	}
	return nil
}

// labelsLangs lists the languages the email templates know.
var labelsLangs = map[string]bool{"en": true, "fr": true}

func atoiOr(val string, fallback int) int {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func parseList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// OriginAllowed reports whether origin may post to this site.
func (s Site) OriginAllowed(origin string) bool {
	for _, o := range s.Origins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}
