go-mibig
========

Go types for [MIBiG](https://mibig.secondarymetabolites.org/) entries:
decoding, encoding and validating MIBiG JSON files according to the current
[MIBiG schema](../schema).

* Covers the full schema, including the parts the data does not use yet, plus
  the fields the data and the Python implementation use beyond it.
* Lossless: decoding and re-encoding any entry of the MIBiG 5.0rc1 release gives
  the same JSON document, apart from key order and whitespace. Unknown keys,
  empty values and legacy formats are preserved.
* Validation collects all issues, each with a JSON path, a stable code and a
  severity, so that command line tools can report, filter and fix them.
* Writes files in the same layout as the Python tooling.
* No dependencies outside the standard library. Needs Go 1.24 or later.

Importing older (v3/v4) data formats is not supported; use
[python-mibig](../python-mibig) for that.


Usage
-----

```go
import mibig "go.kblin.dev/go-mibig"
```

Read, validate and write an entry:

```go
entry, err := mibig.ReadFile("BGC0000001.json")
if err != nil {
	log.Fatal(err) // invalid JSON, or values of the wrong JSON type
}

for _, issue := range entry.Validate() {
	fmt.Println(issue) // loci[0].evidence: error: evidence is required ... [missing-evidence]
}

entry.Comment = "Checked"
if err := mibig.WriteFile("BGC0000001.json", entry); err != nil {
	log.Fatal(err)
}
```

`entry.Validate()` checks at the entry's own quality level;
`entry.ValidateAt(mibig.QualityHigh)` checks whether the entry would qualify
for a higher level. `issues.Err()` returns an error only if there are errors,
not just warnings.

Biosynthetic classes, modules and domains are interfaces, with one struct per
variant:

```go
for _, module := range entry.Biosynthesis.Modules {
	switch m := module.(type) {
	case *mibig.NRPSType1Module:
		if m.ADomain == nil {
			continue
		}
		for _, substrate := range m.ADomain.Substrates {
			fmt.Println(m.Name, substrate.Name)
		}
	case *mibig.UnknownModule:
		fmt.Println("unknown module type", m.Type)
	}
}
```

Build entries in Go and encode them:

```go
entry := &mibig.Entry{
	Accession: "BGC0099999",
	Biosynthesis: mibig.Biosynthesis{
		Classes: mibig.Classes{&mibig.TerpeneClass{Subclass: "Diterpene"}},
	},
	Compounds: []mibig.Compound{{
		Name: "exampline",
		Bioactivities: []mibig.Bioactivity{{
			Name:     mibig.BioactivityName{Activity: "antibacterial"},
			Observed: mibig.Bool(true),
		}},
	}},
	// ...
}
data, err := mibig.Marshal(entry)
```

See the [package documentation](https://pkg.go.dev/secondarymetabolites.org/go-mibig)
and [example_test.go](example_test.go) for more.


Things to know
--------------

* **Unknown keys and empty values** end up in the `Extra` field of each struct
  and are written back. The validator reports them (`unknown-field`,
  `empty-value`). Unknown keys are in `Extra.Unknown`. Empty values such as
  `"comment": null` are written back for as long as the field holds its zero
  value, even after setting and clearing it; call `Extra.Discard("comment")`
  to delete the key instead.
* **Legacy formats** are kept as they are and reported as warnings
  (`legacy-format`): bioactivity names stored as objects
  (`BioactivityName.Legacy`), RiPP core sequences stored as lists
  (`CoreSequence.Legacy`), and the terpene `synthases` key
  (`TerpeneClass.Synthases`).
* **Booleans are pointers** (`*bool`) to tell "false" from "missing";
  `mibig.Bool(true)` fills them in. The compound mass is a `json.Number`, so
  that it is written back exactly as read.
* Where the schema and the data spell things differently (e.g. `release_type`
  vs. `release_types`), the data's spelling is written and both are read.
* `Marshal`/`WriteFile` write two-space indented JSON with non-ASCII
  characters escaped, like Python's `json.dumps(entry, indent=2)`. Keys are
  written in a canonical order, so the first rewrite of an existing file may
  reorder keys.

The differences between the schema, the Python implementation and the data,
and the validation rules, are described in
[docs/schema-notes.md](docs/schema-notes.md). The design is described in
[docs/design.md](docs/design.md).


Development
-----------

```sh
go test ./...
```

To run the tests against a full data release, point `MIBIG_DATA_DIR` at a
directory of entry files:

```sh
MIBIG_DATA_DIR=../v5_data/mibig-5.0rc1-data go test ./...
```

This checks that every entry round-trips, and compares the validation issues
with the baseline in `testdata/data-issues.golden` (made for MIBiG 5.0rc1).
After changing validation rules, or for a new data release, review the changes
and update the golden files with `-update`:

```sh
MIBIG_DATA_DIR=../v5_data/mibig-5.0rc1-data go test ./... -update
git diff testdata/
```

Fuzz the parser with `go test -fuzz FuzzParse`.


License
-------

Apache License 2.0, like the MIBiG API. See [LICENSE](LICENSE). The MIBiG
entries in `testdata/` are MIBiG data, see
[testdata/README.md](testdata/README.md).
