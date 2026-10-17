package mibig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

// Parse decodes a MIBiG entry. It only fails for input that cannot be
// represented: invalid JSON, values of the wrong JSON type, duplicate keys,
// null elements in lists, or a wrong "type" in a slot that allows only one
// domain type. Unknown keys and values are kept and reported by
// Entry.Validate. Decoding errors are *DecodeError or *json.SyntaxError.
func Parse(data []byte) (*Entry, error) {
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, &DecodeError{Err: fmt.Errorf("expected a JSON object, got %s", describeJSON(data))}
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Marshal encodes v, usually an *Entry, in the layout of the MIBiG data
// files, which is the layout of Python's json.dumps(v, indent=2): two space
// indentation, no HTML escaping, non-ASCII characters escaped as \uXXXX and
// no trailing newline.
//
// The standard json.Marshal also works on all types of this package, but
// produces compact output with HTML escaping.
func Marshal(v any) ([]byte, error) {
	compact, err := marshalNoEscape(v)
	if err != nil {
		return nil, err
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact, "", "  "); err != nil {
		return nil, err
	}
	return escapeNonASCII(indented.Bytes()), nil
}

// escapeNonASCII escapes all bytes outside of printable ASCII the way
// Python's json module does with ensure_ascii=True. In valid JSON, these
// bytes only occur in strings.
func escapeNonASCII(data []byte) []byte {
	if !hasNonASCII(data) {
		return data
	}
	out := make([]byte, 0, len(data)+len(data)/8)
	for i := 0; i < len(data); {
		b := data[i]
		if b < utf8.RuneSelf && b != 0x7f {
			out = append(out, b)
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		i += size
		if r >= 0x10000 {
			high, low := utf16.EncodeRune(r)
			out = fmt.Appendf(out, `\u%04x\u%04x`, high, low)
			continue
		}
		out = fmt.Appendf(out, `\u%04x`, r)
	}
	return out
}

func hasNonASCII(data []byte) bool {
	for _, b := range data {
		if b >= 0x7f {
			return true
		}
	}
	return false
}
