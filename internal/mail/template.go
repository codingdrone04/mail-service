package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/codingdrone04/mail-service/internal/validator"
)

const htmlTpl = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="max-width:600px;margin:20px auto;background:#ffffff;">
    <tr>
      <td style="background:#c4a574;padding:20px 30px;">
        <h1 style="margin:0;color:#ffffff;font-size:20px;">New Contact Form Message</h1>
      </td>
    </tr>
    <tr>
      <td style="padding:30px;">
        <table width="100%" cellpadding="0" cellspacing="0">
          <tr>
            <td style="padding:8px 0;color:#666;font-size:14px;width:80px;vertical-align:top;"><strong>From:</strong></td>
            <td style="padding:8px 0;font-size:14px;">{{.Name}}</td>
          </tr>
          <tr>
            <td style="padding:8px 0;color:#666;font-size:14px;vertical-align:top;"><strong>Email:</strong></td>
            <td style="padding:8px 0;font-size:14px;"><a href="mailto:{{.Email}}" style="color:#c4a574;">{{.Email}}</a></td>
          </tr>
          {{if .Subject}}
          <tr>
            <td style="padding:8px 0;color:#666;font-size:14px;vertical-align:top;"><strong>Subject:</strong></td>
            <td style="padding:8px 0;font-size:14px;">{{.Subject}}</td>
          </tr>
          {{end}}
        </table>
        <hr style="border:none;border-top:1px solid #eee;margin:20px 0;">
        <div style="font-size:14px;line-height:1.6;color:#333;white-space:pre-wrap;">{{.Message}}</div>
      </td>
    </tr>
    <tr>
      <td style="padding:15px 30px;background:#fafafa;border-top:1px solid #eee;">
        <p style="margin:0;font-size:12px;color:#999;">Sent at {{.Timestamp}}</p>
      </td>
    </tr>
  </table>
</body>
</html>`

var parsedHTMLTpl = template.Must(template.New("email").Parse(htmlTpl))

type templateData struct {
	Name      string
	Email     string
	Subject   string
	Message   string
	Timestamp string
}

func BuildEmailBody(form validator.ContactForm) (string, error) {
	data := templateData{
		Name:      form.Name,
		Email:     form.Email,
		Subject:   form.Subject,
		Message:   form.Message,
		Timestamp: time.Now().Format("2006-01-02 15:04:05 MST"),
	}
	var buf bytes.Buffer
	if err := parsedHTMLTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func BuildPlainTextBody(form validator.ContactForm) string {
	subject := form.Subject
	if subject == "" {
		subject = "(no subject)"
	}
	return fmt.Sprintf(
		"New Contact Form Message\n\nFrom: %s\nEmail: %s\nSubject: %s\n\n%s\n\n---\nSent at %s",
		form.Name,
		form.Email,
		subject,
		form.Message,
		time.Now().Format("2006-01-02 15:04:05 MST"),
	)
}
