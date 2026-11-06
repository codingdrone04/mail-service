package mail

import (
	"bufio"
	"io"
	"mime/quotedprintable"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/validator"
)

func testSite() config.Site {
	loc, _ := time.LoadLocation("Europe/Paris")
	return config.Site{ID: "mon-site", Name: "Mon Site", To: []string{"contact@example.com"}, SubjectPrefix: "[Mon Site]", Lang: "fr", Location: loc}
}

func form() validator.ContactForm {
	return validator.ContactForm{
		Name: "Jean", Phone: "06 12 34 56 78", Subject: "Rénovation après dégâts des eaux",
		Message: strings.Repeat("Plafond taché, peinture écaillée. ", 10),
		Fields:  validator.Fields{{Label: "Ville", Value: "Anglet"}, {Label: "Type de travaux", Value: "Dégâts des eaux"}},
	}
}

func testMailer() *Mailer {
	return &Mailer{from: "contact@example.org", fromAddr: "contact@example.org", timeout: 2 * time.Second, retries: 1,
		now: func() time.Time { return time.Date(2026, 11, 6, 8, 30, 0, 0, time.UTC) }}
}

func plainPart(t *testing.T, msg string) string {
	start := strings.Index(msg, "quoted-printable\r\n\r\n") + len("quoted-printable\r\n\r\n")
	end := strings.Index(msg[start:], "\r\n------=_Part_") + start
	dec, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(msg[start:end])))
	if err != nil {
		t.Fatal(err)
	}
	return string(dec)
}

func TestMessageEncodingAndHeaders(t *testing.T) {
	m := testMailer()
	msg, err := m.BuildMessage(form(), testSite())
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	for _, h := range []string{"To: contact@example.com", "Date: ", "Message-ID: <", "@example.org>", "X-Mail-Service-Site: mon-site"} {
		if !strings.Contains(s, h) {
			t.Fatalf("missing %q", h)
		}
	}
	if strings.Contains(s, "Reply-To:") {
		t.Fatal("no Reply-To expected without email")
	}
	for _, line := range strings.Split(s, "\r\n") {
		if len(line) > 998 {
			t.Fatal("line longer than RFC 5322 limit")
		}
		for _, r := range line {
			if r > 127 {
				t.Fatalf("raw non-ASCII byte in message: %q", line)
			}
		}
	}
	plain := plainPart(t, s)
	for _, want := range []string{"Téléphone : 06 12 34 56 78", "Ville : Anglet", "E-mail : non renseigné", "écaillée", "Envoyé depuis Mon Site le 06/11/2026 à 09:30"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("plain text missing %q:\n%s", want, plain)
		}
	}
	if strings.Index(plain, "Ville") > strings.Index(plain, "Type de travaux") {
		t.Fatal("fields must keep their order")
	}

	f := form()
	f.Email = "jean@example.com"
	msg, _ = m.BuildMessage(f, testSite())
	if !strings.Contains(string(msg), "Reply-To: jean@example.com") {
		t.Fatal("Reply-To expected when email is given")
	}
}

// fakeSMTP speaks just enough SMTP for net/smtp. failFirstRcpt makes the first
// session answer 451 to RCPT (a temporary error), silent never answers at all.
func fakeSMTP(t *testing.T, failFirstRcpt, silent bool) (port string, got chan string, sessions *int32) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 4)
	sessions = new(int32)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n := atomic.AddInt32(sessions, 1)
			go func(c net.Conn) {
				defer c.Close()
				if silent {
					time.Sleep(5 * time.Second)
					return
				}
				r := bufio.NewReader(c)
				say := func(s string) { c.Write([]byte(s + "\r\n")) }
				say("220 fake")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
						say("250 fake")
					case strings.HasPrefix(cmd, "RCPT") && failFirstRcpt && n == 1:
						say("451 try again later")
					case strings.HasPrefix(cmd, "DATA"):
						say("354 go")
						var b strings.Builder
						for {
							l, err := r.ReadString('\n')
							if err != nil || l == ".\r\n" {
								break
							}
							b.WriteString(l)
						}
						got <- b.String()
						say("250 queued")
					case strings.HasPrefix(cmd, "QUIT"):
						say("221 bye")
						return
					default:
						say("250 ok")
					}
				}
			}(c)
		}
	}()
	_, port, _ = net.SplitHostPort(ln.Addr().String())
	return port, got, sessions
}

func TestDeliverAndRetry(t *testing.T) {
	port, got, sessions := fakeSMTP(t, true, false)
	m := testMailer()
	m.host, m.port = "127.0.0.1", port
	if err := m.Send(form(), testSite()); err != nil {
		t.Fatalf("send with one temporary failure should succeed: %v", err)
	}
	select {
	case msg := <-got:
		if !strings.Contains(msg, "Subject: ") {
			t.Fatal("message not delivered whole")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("nothing delivered")
	}
	if atomic.LoadInt32(sessions) != 2 {
		t.Fatalf("expected 2 sessions (1 retry), got %d", *sessions)
	}
}

func TestDeliverTimeout(t *testing.T) {
	port, _, _ := fakeSMTP(t, false, true)
	m := testMailer()
	m.host, m.port, m.timeout, m.retries = "127.0.0.1", port, 300*time.Millisecond, 0
	start := time.Now()
	if err := m.Send(form(), testSite()); err == nil {
		t.Fatal("silent server must fail")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("timeout not respected: %v", d)
	}
}

// TestWritePreview writes a sample email to $PREVIEW_OUT (make preview).
func TestWritePreview(t *testing.T) {
	out := os.Getenv("PREVIEW_OUT")
	if out == "" {
		t.Skip("set PREVIEW_OUT to write a preview")
	}
	html, err := BuildEmailBody(form(), testSite(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
}
