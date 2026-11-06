package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/validator"
)

// One neutral layout for every project: black on white, system fonts, no brand
// colour. It reads the same in any mail client and in dark mode.
const htmlTpl = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><meta name="color-scheme" content="light dark"><title>{{.Subject}}</title></head>
<body style="margin:0;padding:24px 16px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:15px;line-height:1.5;color:#111111;background:#ffffff;">
<div style="max-width:560px;margin:0 auto;">
<p style="margin:0 0 20px;font-size:17px;font-weight:600;">{{.Heading}}</p>
<table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;">
{{range .Rows}}<tr>
<td style="padding:6px 16px 6px 0;color:#666666;vertical-align:top;white-space:nowrap;">{{.Label}}</td>
<td style="padding:6px 0;vertical-align:top;">{{if .Href}}<a href="{{.Href}}" style="color:#111111;">{{.Value}}</a>{{else}}{{.Value}}{{end}}</td>
</tr>
{{end}}</table>
{{if .Message}}<div style="margin:20px 0 0;padding:16px 0 0;border-top:1px solid #e5e5e5;white-space:pre-wrap;">{{.Message}}</div>{{end}}
<p style="margin:28px 0 0;font-size:12px;color:#888888;">{{.Footer}}</p>
</div>
</body>
</html>`

var parsedHTMLTpl = template.Must(template.New("email").Parse(htmlTpl))

type labels struct {
	Name, Phone, Email, Subject, None, NoSubject, SentFrom, SentAt, Message string
	DateFormat                                                              string
}

var labelsByLang = map[string]labels{
	"en": {Name: "Name", Phone: "Phone", Email: "Email", Subject: "Subject", None: "not provided", NoSubject: "New message",
		SentFrom: "Sent from %s on %s", SentAt: "Sent on %s", Message: "Message", DateFormat: "Jan 2, 2006 at 15:04 MST"},
	"fr": {Name: "Nom", Phone: "Téléphone", Email: "E-mail", Subject: "Objet", None: "non renseigné", NoSubject: "Nouveau message",
		SentFrom: "Envoyé depuis %s le %s", SentAt: "Envoyé le %s", Message: "Message", DateFormat: "02/01/2006 à 15:04"},
}

func labelsFor(site config.Site) labels {
	if l, ok := labelsByLang[site.Lang]; ok {
		return l
	}
	return labelsByLang["en"]
}

type row struct {
	Label, Value string
	Href         template.URL
}

type content struct {
	Lang, Subject, Heading, Message, Footer string
	Rows                                    []row
}

// Subject returns the full email subject for a form.
func Subject(form validator.ContactForm, site config.Site) string {
	s := form.Subject
	if s == "" {
		s = labelsFor(site).NoSubject
	}
	return strings.TrimSpace(site.SubjectPrefix + " " + s)
}

func build(form validator.ContactForm, site config.Site, now time.Time) content {
	l := labelsFor(site)
	loc := site.Location
	if loc == nil {
		loc = time.UTC
	}
	date := now.In(loc).Format(l.DateFormat)
	footer := fmt.Sprintf(l.SentAt, date)
	if site.Name != "" {
		footer = fmt.Sprintf(l.SentFrom, site.Name, date)
	}

	rows := []row{{Label: l.Name, Value: form.Name}}
	if form.Phone != "" {
		tel := strings.NewReplacer(" ", "", ".", "", "(", "", ")", "", "-", "").Replace(form.Phone)
		rows = append(rows, row{Label: l.Phone, Value: form.Phone, Href: template.URL("tel:" + tel)})
	}
	if form.Email != "" {
		rows = append(rows, row{Label: l.Email, Value: form.Email, Href: template.URL("mailto:" + form.Email)})
	} else {
		rows = append(rows, row{Label: l.Email, Value: l.None})
	}
	for _, f := range form.Fields {
		rows = append(rows, row{Label: f.Label, Value: f.Value})
	}
	heading := form.Subject
	if heading == "" {
		heading = l.NoSubject
	}
	return content{Lang: site.Lang, Subject: Subject(form, site), Heading: heading, Message: form.Message, Footer: footer, Rows: rows}
}

func BuildEmailBody(form validator.ContactForm, site config.Site, now time.Time) (string, error) {
	var buf bytes.Buffer
	if err := parsedHTMLTpl.Execute(&buf, build(form, site, now)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func BuildPlainTextBody(form validator.ContactForm, site config.Site, now time.Time) string {
	c := build(form, site, now)
	var b strings.Builder
	b.WriteString(c.Heading + "\n\n")
	sep := ": "
	if site.Lang == "fr" {
		sep = " : " // French typography
	}
	for _, r := range c.Rows {
		b.WriteString(r.Label + sep + r.Value + "\n")
	}
	if c.Message != "" {
		b.WriteString("\n" + c.Message + "\n")
	}
	b.WriteString("\n-- \n" + c.Footer + "\n")
	return b.String()
}
