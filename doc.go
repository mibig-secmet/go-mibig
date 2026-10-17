// Package mibig implements the MIBiG data format: Go types for MIBiG entries,
// decoding and encoding them, and validating them against the MIBiG schema.
//
// # Reading and writing
//
// [ReadFile] and [Parse] decode an [Entry]. Decoding is lossless: keys that are
// not part of the schema, empty values and legacy formats are kept in the
// Extra field of each struct (see [Extras]) and written back by [Marshal] and
// [WriteFile], which produce the same layout as the Python tooling. An empty
// value such as "comment": null is written back while the field holds its
// zero value; [Extras.Discard] drops it, e.g. to delete the key. Decoding
// only fails for input that cannot be represented, e.g. invalid JSON, values
// of the wrong JSON type or duplicate keys (see [DecodeError]); all other
// problems are left to validation.
//
// # Validation
//
// [Entry.Validate] checks an entry against the schema and the semantic rules
// of MIBiG at the entry's quality level, and returns all [Issues] found. Each
// [Issue] has a path, a stable [IssueCode] and a [Severity]. Entries of
// quality questionable are held to fewer rules, e.g. they may lack citations;
// [Entry.ValidateAt] validates at another quality level.
//
// # Polymorphic objects
//
// Biosynthetic classes, modules and domains are represented by the interfaces
// [Class], [Module] and [Domain], with one struct per variant, e.g.
// [NRPSClass], [PKSModularModule] or [AdenylationDomain]. Unknown variants
// decode into [UnknownClass], [UnknownModule] and [UnknownDomain].
//
// # Optional values
//
// Booleans are pointers, so that false and missing can be told apart; [Bool]
// helps filling them in. Optional integers where zero is a valid value are
// pointers too. Optional strings are plain strings, empty when missing.
package mibig
