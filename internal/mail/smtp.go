package mail

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/validator"
)

// Sender sends one contact form for a given site. The handler depends on this
// interface so it can be tested without an SMTP server.
type Sender interface {
	Send(form validator.ContactForm, site config.Site) error
}

type Mailer struct {
	host     string
	port     string
	user     string
	password string
	from     string
	fromAddr string
	timeout  time.Duration
	retries  int
	now      func() time.Time
}

func NewMailer(cfg config.Config) *Mailer {
	addr := cfg.MailFrom
	if a, err := mail.ParseAddress(cfg.MailFrom); err == nil {
		addr = a.Address
	}
	return &Mailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		user:     cfg.SMTPUser,
		password: cfg.SMTPPassword,
		from:     cfg.MailFrom,
		fromAddr: addr,
		timeout:  cfg.SMTPTimeout,
		retries:  1,
		now:      time.Now,
	}
}

// qp encodes a body part as quoted-printable, as its Content-Transfer-Encoding header says.
func qp(s string) string {
	var buf bytes.Buffer
	w := quotedprintable.NewWriter(&buf)
	w.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")))
	w.Close()
	return buf.String()
}

func messageID(domain string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return "<" + hex.EncodeToString(b) + "@" + domain + ">"
}

// BuildMessage returns the full RFC 5322 message for a form.
func (m *Mailer) BuildMessage(form validator.ContactForm, site config.Site) ([]byte, error) {
	now := m.now()
	htmlBody, err := BuildEmailBody(form, site, now)
	if err != nil {
		return nil, fmt.Errorf("building email body: %w", err)
	}
	plainBody := BuildPlainTextBody(form, site, now)

	domain := "localhost"
	if i := strings.LastIndex(m.fromAddr, "@"); i >= 0 {
		domain = m.fromAddr[i+1:]
	}
	boundary := "----=_Part_" + strings.Trim(messageID("x"), "<>@x")

	headers := []string{
		"From: " + m.from,
		"To: " + strings.Join(site.To, ", "),
		"Date: " + now.Format(time.RFC1123Z),
		"Message-ID: " + messageID(domain),
	}
	// Email is optional for some sites: only set Reply-To when we have a valid one.
	if form.Email != "" {
		headers = append(headers, "Reply-To: "+form.Email)
	}
	headers = append(headers,
		"Subject: "+mime.QEncoding.Encode("utf-8", Subject(form, site)),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", boundary),
		"X-Mail-Service-Site: "+siteLabel(site),
	)

	var body strings.Builder
	body.WriteString(strings.Join(headers, "\r\n"))
	body.WriteString("\r\n\r\n")
	for _, part := range []struct{ ctype, text string }{{"text/plain", plainBody}, {"text/html", htmlBody}} {
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString("Content-Type: " + part.ctype + "; charset=utf-8\r\n")
		body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		body.WriteString(qp(part.text))
		body.WriteString("\r\n")
	}
	body.WriteString("--" + boundary + "--\r\n")
	return []byte(body.String()), nil
}

func siteLabel(s config.Site) string {
	if s.ID == "" {
		return "default"
	}
	return s.ID
}

func (m *Mailer) Send(form validator.ContactForm, site config.Site) error {
	msg, err := m.BuildMessage(form, site)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		err = m.deliver(site.To, msg)
		if err == nil || attempt >= m.retries || !temporary(err) {
			return err
		}
		time.Sleep(time.Second)
	}
}

// temporary reports whether a retry may help: network errors and SMTP 4xx replies.
func temporary(err error) bool {
	var tp *textproto.Error
	if errors.As(err, &tp) {
		return tp.Code >= 400 && tp.Code < 500
	}
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, net.ErrClosed)
}

// deliver sends msg over one SMTP session, bounded by m.timeout end to end.
// It does what smtp.SendMail does (STARTTLS when offered, then AUTH) but never hangs.
func (m *Mailer) deliver(to []string, msg []byte) error {
	addr := net.JoinHostPort(m.host, m.port)
	conn, err := net.DialTimeout("tcp", addr, m.timeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(m.timeout))

	var c *smtp.Client
	if m.port == "465" { // implicit TLS
		tc := tls.Client(conn, &tls.Config{ServerName: m.host})
		if c, err = smtp.NewClient(tc, m.host); err != nil {
			return fmt.Errorf("smtp handshake: %w", err)
		}
	} else {
		if c, err = smtp.NewClient(conn, m.host); err != nil {
			return fmt.Errorf("smtp handshake: %w", err)
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err = c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	defer c.Close()

	if m.user != "" {
		if err = c.Auth(smtp.PlainAuth("", m.user, m.password, m.host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err = c.Mail(m.fromAddr); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for i, rcpt := range to {
		if err = c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT TO (recipient %d of %d): %w", i+1, len(to), err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("smtp end of data: %w", err)
	}
	// The server has accepted the message: a failing QUIT must not trigger a
	// retry, which would deliver it twice.
	c.Quit()
	return nil
}
