package mibig

import (
	"encoding/json"
	"slices"
	"sort"
)

// This file supports tools that build or edit entries interactively: the
// vocabularies of the controlled fields of an entry, and creating and decoding
// the variants of classes, modules and domains.

// Vocabularies returns the allowed values of the controlled fields of the
// entry, by path, e.g. "loci[0].evidence[1].method" or
// "biosynthesis.classes[0].subclass". It covers the fields that exist in the
// entry, including empty ones; the vocabulary of a field can depend on its
// context, e.g. on the type of the domain it belongs to. The vocabularies are
// the ones the validator checks against, at the strictest quality level.
func (e *Entry) Vocabularies() map[string][]string {
	v := &validator{quality: QualityHigh, vocabs: map[string][]string{}}
	e.validate(v)
	vocabs := make(map[string][]string, len(v.vocabs))
	for path, vocab := range v.vocabs {
		vocabs[path] = slices.Clone(vocab)
	}
	return vocabs
}

// ClassNames returns the known biosynthetic classes.
func ClassNames() []ClassName { return tags(classUnion, Class.ClassName) }

// ModuleTypes returns the known module types.
func ModuleTypes() []ModuleType { return tags(moduleUnion, Module.ModuleType) }

// DomainTypes returns the known domain types.
func DomainTypes() []DomainType { return tags(domainUnion, Domain.DomainType) }

// NewClass returns a new, empty class of the given name, or nil if the name
// is unknown.
func NewClass(name ClassName) Class { return newVariant(classUnion, name) }

// NewModule returns a new, empty module of the given type, or nil if the type
// is unknown.
func NewModule(moduleType ModuleType) Module { return newVariant(moduleUnion, moduleType) }

// NewDomain returns a new, empty domain of the given type, or nil if the type
// is unknown.
func NewDomain(domainType DomainType) Domain { return newVariant(domainUnion, domainType) }

// UnmarshalClass decodes a single class. Unknown classes decode into an
// *UnknownClass.
func UnmarshalClass(data []byte) (Class, error) { return classUnion.decodeOne(data) }

// UnmarshalModule decodes a single module. Unknown module types decode into
// an *UnknownModule.
func UnmarshalModule(data []byte) (Module, error) { return moduleUnion.decodeOne(data) }

// UnmarshalDomain decodes a single domain. Unknown domain types decode into
// an *UnknownDomain.
func UnmarshalDomain(data []byte) (Domain, error) { return domainUnion.decodeOne(data) }

// tags returns the discriminator values written by the variants of the
// family, sorted, without aliases.
func tags[K ~string, T any](u *union[T], tagOf func(T) K) []K {
	var tags []K
	for tag, newVariant := range u.variants {
		if string(tagOf(newVariant())) == tag {
			tags = append(tags, K(tag))
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	return tags
}

func newVariant[T any, K ~string](u *union[T], tag K) T {
	var zero T
	newVariant, ok := u.variants[string(tag)]
	if !ok {
		return zero
	}
	return newVariant()
}

// decodeOne decodes a single polymorphic object, with the same error
// reporting as decoding an entry.
func (u *union[T]) decodeOne(data []byte) (T, error) {
	var zero T
	if !json.Valid(data) {
		var probe any
		return zero, &DecodeError{Err: json.Unmarshal(data, &probe)}
	}
	value, err := u.decode(data)
	if err != nil {
		return zero, wrapPath("", err)
	}
	return value, nil
}
