package mibig

//go:generate go run ./internal/genjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// This file contains the reflection based JSON object codec that all schema
// structs delegate their MarshalJSON / UnmarshalJSON methods to. See
// docs/design.md for the reasoning. In short, on top of encoding/json it
//
//   - collects unknown keys and the input form of empty values into an Extras
//     field, so decoding and encoding is lossless (see extras.go),
//   - accepts alternative key spellings (`mibig:"alias=other_name"`),
//   - accepts a single value for a list field (`mibig:"single"`),
//   - writes selected fields after all others (`mibig:"last"`),
//   - treats fields that are always written as optional (`mibig:"optional"`),
//   - marks optional lists that must not be empty (`mibig:"nonempty"`), for
//     validation,
//   - writes and checks discriminator keys such as "type" and "class",
//   - reports decode errors with a JSON path.
//
// Only the `omitzero` option of the json struct tag is supported. The struct
// metadata is in codec_meta.go, the decoding of polymorphic objects in
// codec_union.go and the construction of error paths in path.go.

// DecodeError is returned when JSON data cannot be decoded. Path is the JSON
// path of the offending value, e.g. "biosynthesis.modules[2].a_domain".
type DecodeError struct {
	Path string
	Err  error
}

func (e *DecodeError) Error() string {
	if e.Path == "" {
		return e.Err.Error()
	}
	return e.Path + ": " + e.Err.Error()
}

func (e *DecodeError) Unwrap() error {
	return e.Err
}

func isNull(data []byte) bool {
	return string(bytes.TrimSpace(data)) == "null"
}

// decodeObject decodes the JSON object in data into the struct pointed to by
// v. If the type has a discriminator, its key must be present and have one of
// the accepted values.
//
// Like encoding/json, a top-level null leaves the value unchanged; null field
// values and list elements are decoded as zero values by decodeValue.
func decodeObject(data []byte, v any) error {
	if isNull(data) {
		return nil
	}
	rv := reflect.ValueOf(v).Elem()
	info := getStructInfo(rv.Type())
	disc := info.disc
	// Decoding replaces the whole value, including its Extras.
	rv.SetZero()

	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return &DecodeError{Err: err}
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return &DecodeError{Err: fmt.Errorf("expected a JSON object, got %s", describeJSON(data))}
	}

	var extras Extras
	seenDisc := false
	seenKey := make(map[int]string)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return &DecodeError{Err: err}
		}
		key := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return wrapKey(key, err)
		}

		if disc != nil && key == disc.key {
			if seenDisc {
				return wrapKey(key, errors.New("duplicate key"))
			}
			var value string
			if err := json.Unmarshal(raw, &value); err != nil || !disc.accepts(value) {
				return wrapKey(key, fmt.Errorf("expected %q, got %s", disc.values[0], describeJSON(raw)))
			}
			seenDisc = true
			continue
		}

		pos, known := info.byKey[key]
		if !known {
			extras.Unknown = append(extras.Unknown, UnknownField{Key: key, Value: raw})
			continue
		}
		if previous, seen := seenKey[pos]; seen {
			if previous == key {
				return wrapKey(key, errors.New("duplicate key"))
			}
			return wrapKey(key, fmt.Errorf("conflicts with %q, which is the same field", previous))
		}
		seenKey[pos] = key
		fi := &info.fields[pos]
		fv := rv.FieldByIndex(fi.index)
		if err := decodeValue(raw, fv, fi.single); err != nil {
			return wrapKey(key, err)
		}
		if isZero(fv) && !reproduces(fi, fv, raw) {
			extras.preserved = append(extras.preserved, preservedField{field: fi.name, key: key, aliases: fi.aliases, value: raw})
		}
	}
	if _, err := dec.Token(); err != nil {
		return &DecodeError{Err: err}
	}

	if disc != nil && !seenDisc {
		return &DecodeError{Err: fmt.Errorf("missing %q key, expected %q", disc.key, disc.values[0])}
	}
	for pos := range info.fields {
		fi := &info.fields[pos]
		if _, seen := seenKey[pos]; !seen && !fi.omitZero && isZero(rv.FieldByIndex(fi.index)) {
			extras.preserved = append(extras.preserved, preservedField{field: fi.name, key: fi.name, aliases: fi.aliases})
		}
	}
	if info.extras != nil {
		rv.FieldByIndex(info.extras).Set(reflect.ValueOf(extras))
	}
	return nil
}

// reproduces reports whether encoding the zero valued field fv yields raw.
func reproduces(fi *fieldInfo, fv reflect.Value, raw json.RawMessage) bool {
	if fi.omitZero {
		return false
	}
	encoded, err := encodeField(fv)
	if err != nil {
		return false
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		return false
	}
	return bytes.Equal(encoded, compacted.Bytes())
}

// decodeValue decodes raw into the value fv. Lists are decoded element by
// element, so that errors carry the index. If single is set, a non-list value
// is accepted as a list with one element.
func decodeValue(raw json.RawMessage, fv reflect.Value, single bool) error {
	if isNull(raw) {
		fv.SetZero()
		return nil
	}

	if fv.Kind() == reflect.Interface {
		decode, ok := unionDecoders[fv.Type()]
		if !ok {
			return fmt.Errorf("mibig: no decoder for interface type %s", fv.Type())
		}
		value, err := decode(raw)
		if err != nil {
			return err
		}
		fv.Set(reflect.ValueOf(value))
		return nil
	}

	if fv.Type() == numberType {
		// encoding/json would accept "123" and write it back as 123
		if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '"' {
			return fmt.Errorf("expected a number, got %s", describeJSON(raw))
		}
	}

	isList := fv.Kind() == reflect.Slice && fv.Type() != rawMessageType &&
		!reflect.PointerTo(fv.Type()).Implements(unmarshalerType)
	if !isList {
		return json.Unmarshal(raw, fv.Addr().Interface())
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		if !single {
			return fmt.Errorf("expected a JSON array, got %s", describeJSON(raw))
		}
		list := reflect.MakeSlice(fv.Type(), 1, 1)
		if err := decodeValue(raw, list.Index(0), false); err != nil {
			return wrapIndex(0, err)
		}
		fv.Set(list)
		return nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	list := reflect.MakeSlice(fv.Type(), len(items), len(items))
	for i, item := range items {
		element := list.Index(i)
		switch {
		case element.Type() == rawMessageType:
			element.Set(reflect.ValueOf(append(json.RawMessage(nil), item...)))
		case isNull(item):
			// The zero value would be written back as e.g. "" or {}, so
			// null elements cannot be kept.
			return wrapIndex(i, errors.New("null is not allowed in this list"))
		default:
			if err := decodeValue(item, element, false); err != nil {
				return wrapIndex(i, err)
			}
		}
	}
	fv.Set(list)
	return nil
}

// describeJSON gives a short description of a JSON value for error messages.
func describeJSON(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "nothing"
	}
	switch trimmed[0] {
	case '{':
		return "an object"
	case '[':
		return "an array"
	case '"':
		if len(trimmed) > 40 {
			return string(trimmed[:37]) + `..."`
		}
		return string(trimmed)
	default:
		if len(trimmed) > 40 {
			return string(trimmed[:40]) + "..."
		}
		return string(trimmed)
	}
}

// encodeObject encodes the struct v (or pointer to struct) as a JSON object.
// If the type has a discriminator, it is written first.
func encodeObject(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	info := getStructInfo(rv.Type())

	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	writeKey := func(quoted []byte) {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.Write(quoted)
		buf.WriteByte(':')
	}

	if disc := info.disc; disc != nil {
		key, _ := marshalNoEscape(disc.key)
		value, _ := marshalNoEscape(disc.values[0])
		writeKey(key)
		buf.Write(value)
	}

	var extras Extras
	if info.extras != nil {
		extras = rv.FieldByIndex(info.extras).Interface().(Extras)
	}

	for _, pos := range info.order {
		fi := &info.fields[pos]
		fv := rv.FieldByIndex(fi.index)
		if isZero(fv) {
			if p, ok := extras.preservedFor(fi); ok {
				if p.value != nil {
					key, _ := marshalNoEscape(p.key)
					writeKey(key)
					buf.Write(p.value)
				}
				continue
			}
			if fi.omitZero {
				continue
			}
		}
		encoded, err := encodeField(fv)
		if err != nil {
			return nil, wrapKey(fi.name, err)
		}
		writeKey(fi.quoted)
		buf.Write(encoded)
	}

	for _, unknown := range extras.Unknown {
		key, _ := marshalNoEscape(unknown.Key)
		writeKey(key)
		if len(unknown.Value) == 0 {
			buf.WriteString("null")
		} else {
			buf.Write(unknown.Value)
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeField encodes a single field value. Lists are encoded element by
// element, so that errors carry the index. Lists that are always written are
// written as empty lists rather than null.
func encodeField(fv reflect.Value) ([]byte, error) {
	isList := fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() != reflect.Uint8 &&
		!fv.Type().Implements(marshalerType)
	if !isList {
		return marshalNoEscape(fv.Interface())
	}
	if fv.IsNil() {
		return []byte("[]"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i := range fv.Len() {
		if i > 0 {
			buf.WriteByte(',')
		}
		encoded, err := encodeField(fv.Index(i))
		if err != nil {
			return nil, wrapIndex(i, err)
		}
		buf.Write(encoded)
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

type isZeroer interface {
	IsZero() bool
}

func isZero(fv reflect.Value) bool {
	if z, ok := fv.Interface().(isZeroer); ok {
		if fv.Kind() == reflect.Pointer && fv.IsNil() {
			return true
		}
		return z.IsZero()
	}
	return fv.IsZero()
}

// marshalNoEscape is json.Marshal without HTML escaping.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
