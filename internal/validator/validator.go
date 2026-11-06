package validator

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxFields     = 20
	MaxFieldKey   = 50
	MaxFieldValue = 1000
)

// ContactForm is the JSON body accepted by POST /send and /send/{site}.
type ContactForm struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject"`
	Message string `json:"message"`
	Fields  Fields `json:"fields"`  // free project fields, e.g. {"Ville": "Anglet"}, order kept
	Company string `json:"company"` // honeypot field, must stay empty
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Rules says which contact field is mandatory: "email" (default), "phone" or "any" (one of them).
type Rules struct {
	RequireContact string
}

var phoneRe = regexp.MustCompile(`^\+?[0-9 .()\-]{6,30}$`)

// oneLine collapses whitespace and removes CR/LF so header-bound values can never inject headers.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func Sanitize(form *ContactForm) {
	form.Name = oneLine(form.Name)
	form.Email = strings.TrimSpace(form.Email)
	form.Phone = oneLine(form.Phone)
	form.Subject = oneLine(form.Subject)
	form.Message = strings.TrimSpace(form.Message)
	clean := form.Fields[:0]
	for _, f := range form.Fields {
		f.Label, f.Value = oneLine(f.Label), strings.TrimSpace(f.Value)
		if f.Label != "" && f.Value != "" {
			clean = append(clean, f)
		}
	}
	form.Fields = clean
}

// normalizeEmail parses one address (with or without display name) and returns
// the bare address, refusing anything that is not a plain, single-token addr-spec.
func normalizeEmail(raw string) (string, bool) {
	a, err := mail.ParseAddress(raw)
	if err != nil {
		return "", false
	}
	if a.Address == "" || strings.ContainsAny(a.Address, " \t\r\n<>,;\"()[]\\") || strings.Count(a.Address, "@") != 1 {
		return "", false
	}
	return a.Address, true
}

func Validate(form *ContactForm, rules Rules) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) { errs = append(errs, ValidationError{Field: field, Message: msg}) }
	length := utf8.RuneCountInString

	if form.Name == "" {
		add("name", "Name is required")
	} else if length(form.Name) > 100 {
		add("name", "Name must be under 100 characters")
	}

	if form.Email != "" {
		if length(form.Email) > 254 {
			add("email", "Email is too long")
		} else if addr, ok := normalizeEmail(form.Email); !ok {
			add("email", "Invalid email address")
		} else {
			// "Eve <eve@x.com>" is accepted but only the address is kept: the
			// visitor never controls a display name in our Reply-To header.
			form.Email = addr
		}
	}
	if form.Phone != "" && !phoneRe.MatchString(form.Phone) {
		add("phone", "Invalid phone number")
	}
	switch rules.RequireContact {
	case "phone":
		if form.Phone == "" {
			add("phone", "Phone is required")
		}
	case "any":
		if form.Phone == "" && form.Email == "" {
			add("email", "Email or phone is required")
		}
	default:
		if form.Email == "" {
			add("email", "Email is required")
		}
	}

	if length(form.Subject) > 200 {
		add("subject", "Subject must be under 200 characters")
	}

	// The message is optional when the project sends its own fields.
	if form.Message == "" && len(form.Fields) == 0 {
		add("message", "Message is required")
	} else if length(form.Message) > 5000 {
		add("message", "Message must be under 5000 characters")
	}

	if len(form.Fields) > MaxFields {
		add("fields", "Too many fields (max 20)")
	}
	for _, f := range form.Fields {
		if length(f.Label) > MaxFieldKey {
			add("fields", "Field name too long (max 50 characters)")
		} else if length(f.Value) > MaxFieldValue {
			add("fields", "Field value too long (max 1000 characters): "+f.Label)
		}
	}

	return errs
}
