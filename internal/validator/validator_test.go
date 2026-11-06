package validator

import (
	"encoding/json"
	"strings"
	"testing"
)

func fields(errs []ValidationError) map[string]bool {
	m := map[string]bool{}
	for _, e := range errs {
		m[e.Field] = true
	}
	return m
}

func TestContactRules(t *testing.T) {
	f := ContactForm{Name: "A", Message: "m"}
	if !fields(Validate(&f, Rules{}))["email"] {
		t.Fatal("email should be required by default")
	}
	got := fields(Validate(&f, Rules{RequireContact: "phone"}))
	if got["email"] || !got["phone"] {
		t.Fatalf("phone rule: %v", got)
	}
	if !fields(Validate(&f, Rules{RequireContact: "any"}))["email"] {
		t.Fatal("any rule needs email or phone")
	}
	f.Phone = "06 12 34 56 78"
	for _, r := range []string{"phone", "any"} {
		if errs := Validate(&f, Rules{RequireContact: r}); len(errs) != 0 {
			t.Fatalf("%s: valid form rejected: %v", r, errs)
		}
	}
	f.Phone = "call me maybe"
	if !fields(Validate(&f, Rules{RequireContact: "phone"}))["phone"] {
		t.Fatal("invalid phone accepted")
	}
}

func TestEmailNormalized(t *testing.T) {
	f := ContactForm{Name: "A", Message: "m", Email: "Service client <eve@example.com>"}
	if errs := Validate(&f, Rules{}); len(errs) != 0 || f.Email != "eve@example.com" {
		t.Fatalf("display name should be dropped, got %q %v", f.Email, errs)
	}
	for _, bad := range []string{"not an email", "a@b.fr, c@d.fr", `"a b"@example.com`, "a@b@c.fr"} {
		f := ContactForm{Name: "A", Message: "m", Email: bad}
		if !fields(Validate(&f, Rules{}))["email"] {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestFieldsKeepOrderAndMessageOptional(t *testing.T) {
	var f ContactForm
	body := `{"name":"Jean","phone":"0612345678","fields":{"Ville":"Anglet","Type de travaux":"Peinture","Surface":42,"Vide":"  "}}`
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatal(err)
	}
	Sanitize(&f)
	if len(f.Fields) != 3 || f.Fields[0].Label != "Ville" || f.Fields[1].Label != "Type de travaux" || f.Fields[2].Value != "42" {
		t.Fatalf("fields order/clean: %+v", f.Fields)
	}
	if errs := Validate(&f, Rules{RequireContact: "phone"}); len(errs) != 0 {
		t.Fatalf("message should be optional with fields: %v", errs)
	}
	if err := json.Unmarshal([]byte(`{"fields":{"a":{"nested":1}}}`), &f); err == nil {
		t.Fatal("nested field value must be rejected")
	}
	f.Fields = Fields{{Label: "x", Value: strings.Repeat("é", 1001)}}
	if !fields(Validate(&f, Rules{RequireContact: "phone"}))["fields"] {
		t.Fatal("too long field value accepted")
	}
}

func TestSanitizeStripsNewlines(t *testing.T) {
	f := ContactForm{Name: "A\r\nBcc: x@y.z", Subject: "Hi\nthere", Message: " line1\nline2 ", Fields: Fields{{Label: "a\nb", Value: "v"}}}
	Sanitize(&f)
	if f.Name != "A Bcc: x@y.z" || f.Subject != "Hi there" || f.Message != "line1\nline2" || f.Fields[0].Label != "a b" {
		t.Fatalf("sanitize: %+v", f)
	}
}
