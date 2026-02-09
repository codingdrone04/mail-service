package mail

import (
	"fmt"
	"mime"
	"net/smtp"
	"strings"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/validator"
)

type Mailer struct {
	host     string
	port     string
	user     string
	password string
	from     string
	to       string
}

func NewMailer(cfg config.Config) *Mailer {
	return &Mailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		user:     cfg.SMTPUser,
		password: cfg.SMTPPassword,
		from:     cfg.MailFrom,
		to:       cfg.MailTo,
	}
}

func (m *Mailer) Send(form validator.ContactForm) error {
	htmlBody, err := BuildEmailBody(form)
	if err != nil {
		return fmt.Errorf("building email body: %w", err)
	}
	plainBody := BuildPlainTextBody(form)

	subject := form.Subject
	if subject == "" {
		subject = "No Subject"
	}
	subject = "[Portfolio Contact] " + subject

	boundary := "----=_NextPart_ContactForm"

	headers := []string{
		"From: " + m.from,
		"To: " + m.to,
		"Reply-To: " + form.Email,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", boundary),
	}

	var body strings.Builder
	body.WriteString(strings.Join(headers, "\r\n"))
	body.WriteString("\r\n\r\n")

	// Plain text part
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	body.WriteString(plainBody)
	body.WriteString("\r\n\r\n")

	// HTML part
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	body.WriteString(htmlBody)
	body.WriteString("\r\n\r\n")

	body.WriteString("--" + boundary + "--\r\n")

	addr := m.host + ":" + m.port
	auth := smtp.PlainAuth("", m.user, m.password, m.host)

	// net/smtp.SendMail handles STARTTLS automatically (port 587).
	// For implicit TLS (port 465), you'd need crypto/tls + smtp.NewClient instead.
	err = smtp.SendMail(addr, auth, m.from, []string{m.to}, []byte(body.String()))
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}

	return nil
}
