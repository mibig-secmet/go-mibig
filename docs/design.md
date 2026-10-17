# go-mibig design

This document describes how the Go implementation of the MIBiG schema is put
together, and why. For the list of places where the JSON schema, the Python
implementation and the actual data disagree, see
[schema-notes.md](schema-notes.md).

## Goals

1. **Cover the full current schema** (`schema/schemas/**`), including parts the
   data does not use yet (CAL / NRPS type VI / "other" / iterative PKS modules,
   PKS starter units, gene-level domains, assay measurements, ...).
2. **Load every entry of the current data release** (MIBiG 5.0rc1: 3173
   entries) without errors and without losing data: decoding and re-encoding an
   entry yields the same JSON document, apart from key order and whitespace,
   down to how numbers are written.
3. **Validation as a first-class API**, usable by command line tools that check
   and maintain the data. Validation never fails fast; it collects all issues
   with a JSON path, a message and a severity.
4. **Write files the way the existing tooling does** (Python's
   `json.dumps(indent=2)`), so maintenance tools produce small diffs.
5. No third-party dependencies.

Importing older (v3/v4) data formats is a non-goal.

## Package layout

A single package, `mibig`, split into files by schema area:

| File               | Contents                                                     |
| ------------------ | ------------------------------------------------------------ |
| `entry.go`         | `Entry`, `Locus`, `Taxonomy`, quality/status/completeness    |
| `common.go`        | `Citation`, `GeneID`, `SMILES`, `Location`, `Date`, evidence |
| `changelog.go`     | `Changelog`, `Release`, `ReleaseEntry`, `SubmitterID`        |
| `compound.go`      | `Compound`, bioactivities, assays, database IDs              |
| `genes.go`         | `Genes`: additions, deletions, annotations, gene functions   |
| `biosynthesis.go`  | `Biosynthesis`, operons, paths, monomers, release types      |
| `classes.go`       | the `Class` union (NRPS, PKS, ribosomal, ...)                |
| `modules.go`       | the `Module` union (NRPS type I, PKS modular, ...)           |
| `domains.go`       | the `Domain` union (adenylation, ketoreductase, ...)         |
| `vocabulary.go`    | controlled vocabularies (enum value sets)                    |
| `codec.go`         | the reflection based JSON object codec (see below)           |
| `codec_meta.go`    | struct tag options, cached field metadata, discriminators    |
| `codec_union.go`   | decoding of polymorphic objects, `unionSpec` / `newUnion`    |
| `extras.go`        | `Extras`: unknown keys and preserved input forms             |
| `path.go`          | building and parsing paths of decode errors and issues       |
| `format.go`        | `Parse`, `Marshal`                                           |
| `io.go`            | `ReadFile`, atomic `WriteFile`                               |
| `validate.go`      | validation framework: `Issue`, `Issues`, severities          |
| `walk.go`          | reflective walk: unknown keys, `Entry.References`            |
| `json_gen.go`      | the `MarshalJSON` / `UnmarshalJSON` methods, generated       |

Validation rules live next to the types they check, in unexported
`validate(v *validator, path string)` methods.

## Polymorphic objects

Biosynthetic classes (discriminated by `"class"`), modules and domains
(discriminated by `"type"`) are Go interfaces (`Class`, `Module`, `Domain`)
implemented by one struct per variant. Lists of them use the slice types
`Classes`, `Modules` and `Domains`, whose `UnmarshalJSON` dispatches on the
discriminator. Consumers use type switches:

```go
for _, m := range entry.Biosynthesis.Modules {
	switch m := m.(type) {
	case *mibig.NRPSType1Module:
		if m.ADomain != nil {
			fmt.Println(m.ADomain.Substrates)
		}
	}
}
```

The variants of each family are listed once, in a `unionSpec` passed to
`newUnion` (`classUnion`, `moduleUnion`, `domainUnion`). The discriminator
value of a variant is the one its `ClassName` / `ModuleType` / `DomainType`
method returns; `newUnion` derives from the list both the decoding of the
family and the discriminator each variant struct writes and checks. Adding a
variant takes:

1. the struct, with an `Extra Extras` field, its tag method
   (`DomainType()` etc.) and its `validate` method;
2. a constructor in the `variants` list of the family;
3. `go generate`, which writes its JSON methods to `json_gen.go`
   (`internal/genjson`; a test fails if the file is out of date).

Slots that the schema restricts to one domain type (e.g. `a_domain`,
`carriers`, `thioesterases`) use the concrete struct type directly. Each
concrete struct writes its own discriminator when marshalled and checks it when
unmarshalled.

Unknown discriminator values in lists (and in the CAL module's `cal` domain)
decode into `UnknownClass`, `UnknownModule` or `UnknownDomain`, which keep the
raw JSON and are reported by the validator. A missing or wrong discriminator in
a single-type slot (e.g. a condensation domain as `a_domain`) cannot be
represented and is a decode error that names the path. `null` elements of
these lists decode to nil interfaces and are reported by the validator.

## The object codec

`encoding/json` (v1) cannot do the following, all of which this package needs:

* keep unknown keys so that decode → encode is lossless;
* accept a second spelling of a key (the schema spelling, where the data uses a
  different one);
* accept a single value where a list is expected (where the schema has a single
  value and the data a list);
* inject and verify discriminator keys;
* report decode errors with a JSON path.

Rather than hand-writing these per type, every schema struct delegates its
`MarshalJSON` / `UnmarshalJSON` to a small reflection-based codec
(`codec.go`). The methods are one-liners generated into `json_gen.go` for
every struct with an `Extras` field, except where a type declares its own
(e.g. `BioactivityName`). It is driven by the standard `json` struct tag plus an optional
`mibig` tag:

```go
type NRPSClass struct {
	Subclass     string        `json:"subclass"`
	ReleaseTypes []ReleaseType `json:"release_types,omitzero" mibig:"alias=release_type"`
	Extra        Extras        `json:"-"`
}
```

* `json:"name"`: key name. Key order on output is struct field order.
* `omitzero`: omit zero values (nil slices, empty strings, nil pointers).
  Non-nil empty slices are kept, so `[]` round-trips.
* `mibig:"alias=x"`: also accept key `x` when decoding. Giving both spellings
  in one object is a decode error.
* `mibig:"single"`: accept a single value for a list field.
* `mibig:"last"`: write the field after all others (used for fields of
  embedded base structs).
* `mibig:"optional"`: the field is always written, but may be missing.
* `mibig:"nonempty"`: an optional list that must not be empty if present
  (`minItems: 1` in the schema). Only used by validation.
* A field of type `Extras` keeps what the Go fields cannot represent:
  - `Unknown`: unknown keys, in input order, written back after the known
    keys;
  - the preserved input form of known keys: an empty value the field cannot
    express (`"subtype": ""` or `"cyclases": null` on an `omitzero` field,
    `null` for a number), written back in place, or a required key that was
    missing, which stays missing instead of being written as e.g.
    `"name": ""`. Either applies only while the field holds its zero value:
    setting a field and clearing it again brings the input form back.
    `Extras.Preserved` reports the input form, and `Extras.Discard(key)` /
    `Extras.DiscardAll()` drop it, so the field is encoded from its Go value
    alone; e.g. clearing `Comment` and discarding `"comment"` deletes the
    key.

The codec decodes objects with a token stream (to keep key order for
`Extras`), decodes and encodes lists element by element (to report indices in
error paths) and hands everything else to `encoding/json`. Other rules:

* Keys are case sensitive (unlike `encoding/json`). Duplicate known keys,
  including discriminators, are decode errors; duplicate unknown keys are kept.
* `null` elements of typed lists (`"references": [null]`) are decode errors,
  because the zero value would be written back as e.g. `""`.
* Unmarshalling replaces the whole value, including its `Extras`. As usual
  for `UnmarshalJSON`, a top-level `null` leaves the value unchanged; `null`
  field values decode as zero values (and are kept as empty values).
* JSON strings are not accepted for number fields.

## Optional values

* All booleans are `*bool`, so `false` and "missing" stay distinct. `Bool(b)`
  helps filling them in.
* Optional integers where zero is meaningful are `*int`.
* Non-integer numbers (`Compound.Mass`, assay measurements) are `json.Number`,
  so they are written back exactly as read: the data has both `500` and
  `867.0`. Use `Mass.Float64()` for the value.
* Optional strings are plain `string` with `omitzero`. Empty strings and nulls
  found in the input are preserved through `Extras` (see above).

## Legacy and data-only forms

Decisions agreed with the data maintainer:

* Where the schema and the data/Python implementation disagree and the data
  never uses the schema form, **the data form is written** and the schema form
  is accepted when reading (e.g. `release_types` vs `release_type`).
* **Legacy shapes found in older entries are preserved** unchanged and the
  validator reports them as warnings, so tools can migrate them explicitly
  (bioactivity names stored as objects, core sequences stored as lists,
  terpene `synthases` instead of `synthases_cyclases`).

The full list is in [schema-notes.md](schema-notes.md).

## Validation

```go
issues := entry.Validate()          // at the entry's own quality level
issues := entry.ValidateAt(mibig.QualityHigh) // "could this be promoted?"
if err := issues.Err(); err != nil { ... }    // nil unless there are errors
```

Each `Issue` has a `Path` (`biosynthesis.modules[3].a_domain.substrates[0]`),
a stable `Code` for filtering and automated fixes (`required`,
`invalid-value`, `missing-evidence`, `inconsistent`, `duplicate`,
`not-allowed`, `unknown-type`, `unknown-field`, `empty-value`,
`legacy-format`), a `Severity` (`SeverityError` or `SeverityWarning`) and a
`Message`. `Issue.Pointer` gives the location as a JSON Pointer. Keys that are
not plain words are quoted in paths, e.g. `["odd.key"]`.

Decoding errors (`Parse`) and validation issues (`Validate`) are separate:
`Parse` only fails if the input cannot be represented at all (invalid JSON,
wrong JSON types, wrong discriminators in single-type slots, ...).

Rules come from the JSON schema (required keys, controlled vocabularies,
patterns, `minItems`, `uniqueItems`, minimum values) plus the semantic rules
of the Python implementation (details required for "other" values, inactive
domains need evidence, ...). As in the Python implementation, entries at
quality `questionable` are allowed to lack citations and evidence.

Unknown keys, empty values and missing required keys are reported by a
reflective walk over the whole entry: errors for unknown keys with content,
for missing required keys and for required keys whose value had to be dropped
(e.g. `"location": null`); warnings for empty leftovers (`null`, `""`, `[]`,
`{}`) on optional keys, including empty `nonempty` lists, which the
conversions left in many places. The walk skips paths where the validation of
the types already reported an error, so a value is not reported twice.

Cross-checks against GenBank records (gene IDs exist, coordinates within the
sequence) need sequence data and are out of scope for now.

## Output format

`Marshal` and `WriteFile` produce the same layout as Python's
`json.dumps(obj, indent=2)`: two-space indentation, `": "` separators, no
HTML escaping, non-ASCII characters escaped as `\uXXXX`, no trailing newline.
Key order is the canonical struct order, which follows the most common order in
the data. The data uses several key orders for the same kind of object (and
some files are not indented at all), so the first rewrite of a file can
reorder keys; later rewrites are stable. Of the 5.0rc1 files, 70 are already
byte-identical to their rewrite.

The standard `json.Marshal` also works on all types, but writes compact output
with HTML escaping.

`WriteFile` writes a temporary file next to the target, syncs it and renames
it over the target, keeping the target's permissions. The rename is atomic on
Unix systems.

## Testing

* Unit tests for the codec, the formatter (against real Python output) and the
  individual validation rules.
* The usable schema examples (`testdata/schema-examples`) decode into their
  types without unknown keys and validate cleanly at quality high.
* 21 representative real entries (`testdata/entries`) round-trip, and their
  validation issues match reviewed golden files.
* A test that every struct with `Extras` has the codec's JSON methods, and that
  `json_gen.go` is up to date.
* Data release tests, enabled by setting `MIBIG_DATA_DIR`: every entry must
  parse and round-trip to the same document (numbers compared as written),
  a second encoding must be byte-identical, and the validation issues, counted
  by severity, code and path, must match a reviewed baseline.
* A fuzz test for `Parse`: no panics, lossless round trips (apart from the
  documented alias normalisations) and stable encoding.
