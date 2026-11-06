package validator

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Field is one free project field, e.g. {"Ville", "Anglet"}.
type Field struct {
	Label string
	Value string
}

// Fields is a JSON object whose keys keep the order the client sent them in,
// so the email shows them in the order the form shows them.
type Fields []Field

func (f *Fields) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*f = nil
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("fields must be an object of strings")
	}
	out := Fields{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := kt.(string)
		var val any
		if err := dec.Decode(&val); err != nil {
			return err
		}
		switch v := val.(type) {
		case string:
			out = append(out, Field{Label: key, Value: v})
		case float64, bool:
			out = append(out, Field{Label: key, Value: fmt.Sprint(v)})
		case nil:
		default:
			return fmt.Errorf("field %q must be a string", key)
		}
		if len(out) > MaxFields*2 {
			return fmt.Errorf("too many fields")
		}
	}
	*f = out
	return nil
}
