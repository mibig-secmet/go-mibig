package mibig

import (
	"encoding/json"
	"slices"
)

// Extras holds the parts of a JSON object that the Go struct cannot represent,
// so that decoding and re-encoding an entry does not lose data. There are two
// kinds:
//
//   - Unknown holds the keys that are not part of the schema, in input order.
//     They are written back after all known keys. Tools can edit the list
//     directly.
//   - The preserved input form of known keys. Some inputs cannot be told
//     apart from the zero value of the Go field: an empty value the field
//     cannot express (e.g. "subtype": "" or "cyclases": null), or a key that
//     is always written but was missing (e.g. no "name"). Instead of writing
//     the zero value (or omitting it), the encoder reproduces the input form
//     for as long as the Go field holds its zero value. [Extras.Preserved]
//     reports it and [Extras.Discard] drops it, so that the field is encoded
//     from its Go value alone.
//
// The validator reports unknown keys and preserved empty values.
type Extras struct {
	// Unknown holds the keys that are not part of the schema.
	Unknown []UnknownField

	preserved []preservedField
}

// UnknownField is a key that is not part of the schema, and its raw value.
type UnknownField struct {
	Key   string
	Value json.RawMessage
}

// preservedField is the input form of a known key whose Go field holds its
// zero value.
type preservedField struct {
	// field is the canonical key of the field, key the key as written in the
	// input, which can be an alias.
	field, key string
	// aliases are the other accepted keys of the field. Shared with the
	// field metadata; read only.
	aliases []string
	// value is the raw input value, or nil if the key was missing.
	value json.RawMessage
}

// matches reports whether key names the field of p.
func (p *preservedField) matches(key string) bool {
	return key == p.field || slices.Contains(p.aliases, key)
}

// Get returns the raw value of the unknown key, if present.
func (e Extras) Get(key string) (json.RawMessage, bool) {
	for _, field := range e.Unknown {
		if field.Key == key {
			return field.Value, true
		}
	}
	return nil, false
}

// Preserved returns the preserved input form of the known field with the
// given JSON key (or one of its aliases): the raw value, or nil if the key
// was missing from the input. ok is false if nothing is preserved for the
// field. The preserved form is only written while the Go field holds its zero
// value.
func (e Extras) Preserved(key string) (value json.RawMessage, ok bool) {
	for _, p := range e.preserved {
		if p.matches(key) {
			return p.value, true
		}
	}
	return nil, false
}

// Discard drops the preserved input form of the known field with the given
// JSON key (or one of its aliases), so that the field is encoded from its Go
// value alone: a zero value is then omitted if the field is optional
// (omitzero), and written as its zero value (e.g. "" or []) otherwise.
//
// Without Discard, setting a field to its zero value brings back the input
// form, e.g. "comment": null, even if the field was set to another value in
// between.
func (e *Extras) Discard(key string) {
	// Copies of the struct share the backing array, so it is not modified.
	var kept []preservedField
	for _, p := range e.preserved {
		if !p.matches(key) {
			kept = append(kept, p)
		}
	}
	e.preserved = kept
}

// DiscardAll drops the preserved input forms of all known fields, see
// [Extras.Discard]. Unknown keys are kept.
func (e *Extras) DiscardAll() {
	e.preserved = nil
}

// IsZero reports whether there are neither unknown keys nor preserved input
// forms.
func (e Extras) IsZero() bool {
	return len(e.Unknown) == 0 && len(e.preserved) == 0
}

// preservedFor returns the preserved input form of a field.
func (e Extras) preservedFor(fi *fieldInfo) (preservedField, bool) {
	for _, p := range e.preserved {
		if p.field == fi.name {
			return p, true
		}
	}
	return preservedField{}, false
}
