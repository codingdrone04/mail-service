package validator

import (
	"net/mail"
	"strings"
)

type ContactForm struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Message string `json:"message"`
	Company string `json:"company"` // honeypot field
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func Sanitize(form *ContactForm) {
	form.Name = strings.TrimSpace(form.Name)
	form.Email = strings.TrimSpace(form.Email)
	form.Subject = strings.TrimSpace(form.Subject)
	form.Message = strings.TrimSpace(form.Message)
}

func Validate(form *ContactForm) []ValidationError {
	var errs []ValidationError

	if form.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "Name is required"})
	} else if len(form.Name) > 100 {
		errs = append(errs, ValidationError{Field: "name", Message: "Name must be under 100 characters"})
	}

	if form.Email == "" {
		errs = append(errs, ValidationError{Field: "email", Message: "Email is required"})
	} else if len(form.Email) > 254 {
		errs = append(errs, ValidationError{Field: "email", Message: "Email is too long"})
	} else if _, err := mail.ParseAddress(form.Email); err != nil {
		errs = append(errs, ValidationError{Field: "email", Message: "Invalid email address"})
	}

	if len(form.Subject) > 200 {
		errs = append(errs, ValidationError{Field: "subject", Message: "Subject must be under 200 characters"})
	}

	if form.Message == "" {
		errs = append(errs, ValidationError{Field: "message", Message: "Message is required"})
	} else if len(form.Message) > 5000 {
		errs = append(errs, ValidationError{Field: "message", Message: "Message must be under 5000 characters"})
	}

	return errs
}
