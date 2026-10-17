package mibig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// This file holds the decoding of polymorphic objects (classes, modules and
// domains).

// unionDecoders decodes JSON into interface typed fields, e.g. a single Domain.
var unionDecoders = map[reflect.Type]func(json.RawMessage) (any, error){}

// unionSpec defines a family of polymorphic objects. It is the single place
// that lists the variants: newUnion derives the decoding of the family and
// the discriminator each variant writes and checks from it.
type unionSpec[T any, K ~string] struct {
	// key is the discriminator key, e.g. "type".
	key string
	// tagOf returns the discriminator value of a variant, e.g.
	// Domain.DomainType.
	tagOf func(T) K
	// variants create a new value of each variant, which must be a pointer
	// to a struct.
	variants []func() T
	// aliases are further discriminator values accepted when reading, mapped
	// to the value that is written.
	aliases map[string]K
	// unknown creates the value for an unknown discriminator value.
	unknown func(tag K, raw json.RawMessage) T
}

// union decodes one of the polymorphic object families.
type union[T any] struct {
	key      string
	variants map[string]func() T
	unknown  func(tag string, raw json.RawMessage) T
}

// newUnion sets up the polymorphic object family described by spec. It is
// called during package initialisation and panics if spec is inconsistent.
func newUnion[T any, K ~string](spec unionSpec[T, K]) *union[T] {
	u := &union[T]{
		key:      spec.key,
		variants: map[string]func() T{},
		unknown: func(tag string, raw json.RawMessage) T {
			return spec.unknown(K(tag), raw)
		},
	}
	discs := map[K]*discriminator{}
	for _, newVariant := range spec.variants {
		value := newVariant()
		tag := spec.tagOf(value)
		t := reflect.TypeOf(value)
		if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
			panic(fmt.Sprintf("mibig: variant %q is %s, not a pointer to a struct", tag, t))
		}
		if _, dup := u.variants[string(tag)]; dup {
			panic(fmt.Sprintf("mibig: duplicate variant %q", tag))
		}
		if _, dup := discriminators[t.Elem()]; dup {
			panic(fmt.Sprintf("mibig: %s is registered twice", t.Elem()))
		}
		disc := &discriminator{key: spec.key, values: []string{string(tag)}}
		discriminators[t.Elem()] = disc
		discs[tag] = disc
		u.variants[string(tag)] = newVariant
	}
	for alias, tag := range spec.aliases {
		disc, ok := discs[tag]
		if !ok {
			panic(fmt.Sprintf("mibig: alias %q of unknown variant %q", alias, tag))
		}
		if _, dup := u.variants[alias]; dup {
			panic(fmt.Sprintf("mibig: duplicate variant %q", alias))
		}
		disc.values = append(disc.values, alias)
		u.variants[alias] = u.variants[string(tag)]
	}
	unionDecoders[reflect.TypeFor[T]()] = func(raw json.RawMessage) (any, error) {
		return u.decode(raw)
	}
	return u
}

// decode decodes a single polymorphic object.
func (u *union[T]) decode(raw json.RawMessage) (T, error) {
	var zero T
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return zero, fmt.Errorf("expected a JSON object, got %s", describeJSON(raw))
	}
	var tag string
	if rawTag, ok := probe[u.key]; ok {
		// a non-string tag is treated like an unknown one
		_ = json.Unmarshal(rawTag, &tag)
	}
	newVariant, ok := u.variants[tag]
	if !ok {
		// The probe map hides duplicate keys; known variants are checked by
		// decodeObject.
		if countKey(raw, u.key) > 1 {
			return zero, wrapKey(u.key, errors.New("duplicate key"))
		}
		return u.unknown(tag, append(json.RawMessage(nil), raw...)), nil
	}
	value := newVariant()
	if err := json.Unmarshal(raw, value); err != nil {
		return zero, err
	}
	return value, nil
}

// countKey counts the top-level occurrences of key in a valid JSON object.
func countKey(raw json.RawMessage, key string) int {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return 0
	}
	count := 0
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return count
		}
		if tok == key {
			count++
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return count
		}
	}
	return count
}

// decodeList decodes a list of polymorphic objects.
func (u *union[T]) decodeList(data []byte) ([]T, error) {
	if isNull(data) {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, &DecodeError{Err: fmt.Errorf("expected a JSON array, got %s", describeJSON(data))}
	}
	list := make([]T, len(items))
	for i, item := range items {
		if isNull(item) {
			continue // leave the element nil; the validator reports it
		}
		value, err := u.decode(item)
		if err != nil {
			return nil, wrapIndex(i, err)
		}
		list[i] = value
	}
	return list, nil
}
