# Schema notes

The Go types follow three sources, which do not always agree:

* the JSON schema in the [schema repository](../../schema/schemas),
* the Python implementation in
  [python-mibig](../../python-mibig/mibig/converters/shared), which is ahead of
  the schema in several places,
* the data of the MIBiG 5.0rc1 release (3173 entries).

This document lists the differences and how go-mibig handles them. The guiding
decisions were:

* **Data form wins, schema form is accepted.** Where the data never uses the
  schema's form, go-mibig writes the data form and accepts the schema form when
  reading.
* **Legacy shapes are preserved and reported.** Older shapes that newer
  entries no longer use are kept as they are and reported as warnings
  (`legacy-format`), so tools can migrate them explicitly.
* **Leftovers are preserved and reported.** Unknown keys and empty values are
  written back unchanged and reported (`unknown-field`, `empty-value`).

Items marked **schema fix** are probably bugs in the schema repository.

## Bugs in the schema files

| File                                    | Problem                                                                                                        |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `mibig/biosynthesis/biosynthesis.json`  | Invalid JSON: missing comma after `"type": "object"` in the operon items.                                     |
| `mibig/biosynthesis/modules/pks_*.json` | Refer to `domains/ketosynthase.json`, which does not exist. go-mibig has `KetosynthaseDomain`.                  |
| `mibig/entry.json`                      | `version` uses `minValue` instead of `minimum`; `entryId` puts its regex in `format` instead of `pattern`.     |
| `mibig/compound.json`                   | `bioactivity` uses `extra_properties` instead of `additionalProperties`.                                       |
| `mibig/compound.json`                   | `assay` requires `concentration`, which is only defined inside `measurement`.                                  |
| `mibig/gene/domain.json`                | Refers to `/schema/mibig/common/...` instead of `/schemas/...`.                                                |
| `mibig/biosynthesis/saccharide.json`    | `subclass` has an empty `enum`, so no value is valid. go-mibig treats it as free text.                         |
| `common/smiles.json`                    | The pattern is not anchored at the end, so anything starting with a valid character matches.                  |
| several                                 | Inconsistent `$id`s (`schemas/genes.json`, `gene_domains.json`, `.../modules/domains/thioesterase.json`, no `.json`). |
| `modules/common/*.json`                 | `monomer.json` and `non_canonical.json` are defined, but not referenced by any module.                          |

The examples in the schema repository are partly stale (empty `{}` files, path
`steps` as objects, ribosomal `subclass` as an object, evidence with `name`
instead of `method`). The usable ones are in
[`testdata/schema-examples`](../testdata/schema-examples); the saccharide
example got a `"class"` key.

## Different spellings and shapes

go-mibig writes the left column and accepts both.

| Data / Python                              | Schema                                | Where                               |
| ------------------------------------------ | ------------------------------------- | ----------------------------------- |
| `release_types`                            | `release_type`                        | NRPS class                          |
| `db_reference`                             | `dbReference`                         | gene tailoring functions            |
| `nonElongating`                            | `non_elongating`                      | non-canonical module activity       |
| `comment`                                  | `comments`                            | modules                             |
| `"class": "PKS"`                           | `"class": "pks"`                      | PKS class                           |
| evidence `method`                          | evidence `name`                       | glycosyltransferases, non-canonical activity |
| `substrates`: list of SMILES               | `substrate`: one SMILES               | ligase domains                      |
| `evidence`: list                           | `evidence`: one object                | ligase domains, glycosyltransferases |
| `references`: list                         | `references`: one citation            | mutation phenotypes                 |
| `concentration` (free text) and `target`   | `measurement` object, `test_system`, ... | assays (both are supported)      |

Integrated monomers of modules have both `evidence` (from
`modules/common/monomer.json`) and `references` (from
`biosynthesis/common/monomer.json`) in the data, so `Monomer` has both.

## Not in the schema, but in the data and/or Python

All of these are modelled as regular fields:

* entry `legacy_references` (only allowed for questionable entries),
* locus `draft_genome`,
* compound evidence `details`, and the method `"Other"` (which needs details),
* gene annotation `domains`, `mutation_phenotype` and `comment`,
* module `non_canonical_activity`,
* module types `pks-modular-starter` and `pks-trans-at-starter`,
* domain types `amp-binding`, `branching`, `product_template` and
  `ketosynthase`,
* `references` on adenylation, AMP-binding and aminotransferase domains,
* `active` on domain types the Python implementation treats as "active
  domains".

The schema has `genes.domains` (domains per gene list); the data annotates
domains per gene in `genes.annotations[].domains`. Both are supported.

## Controlled vocabularies

[`vocabulary.go`](../vocabulary.go) has the union of the schema's and the
Python implementation's values; the latter added e.g. tailoring functions
"Macrolactam formation" and "Ring contraction", terpene subclass
"Sesterterpene", ketoreductase types "A" and "B", acyltransferase substrates
"acetyl-CoA" and "methoxymalonyl-CoA", many "other" subclasses and compound
classes. The schema's compound classes ("alkaloid", "nucleoside", "peptide")
are not used by the data and are not accepted.

RiPP types accept both "Other" (schema) and "other" (Python). Crosslink types
are lower case in the schema; the data has 32 capitalised values ("Thioether",
...), which are reported.

## Legacy shapes (preserved, reported as warnings)

| Shape                                                     | Current form                         | Entries |
| --------------------------------------------------------- | ------------------------------------ | ------- |
| bioactivity name `{"activity": ..., "comment": ...}`      | a string; the comment goes to details | 1984 names |
| RiPP `core_sequence` as a list                            | a string                             | 38      |
| terpene `synthases`                                       | `synthases_cyclases`                 | 47      |

## Leftovers from the conversion to v4/v5

Empty `cyclases`, `precursors` and `subtype` keys on classes, empty `subtype`
and `references` keys on domains, and empty strings (`"details": ""`, ...) are
kept and reported as warnings.

Optional lists with `minItems: 1` in the schema (`cyclases`, `at_domain`
`substrates` and `evidence`, `integrated_monomers`, `genes.to_add`, ...) are
often present but empty (`[]`) in the data, e.g. 1266 PKS `cyclases` and 641
acyltransferase `evidence` lists. These are reported as warnings
(`empty-value`) rather than errors. Empty optional lists without `minItems`
(`carriers`, `databaseIds`, ...) are valid and not reported.

`null` for a required value (`"location": null`, `"from": null`) is an
error (`required`), even though it decodes as the zero value.

## Validation rules

Rules come from the schema plus the semantic checks of the Python
implementation. Notable decisions:

* **Quality levels.** As in Python, entries of quality `questionable` may lack
  citations and evidence, may have negative placeholder coordinates (-1), may
  have gene IDs with unusual characters (multi-locus names), do not need
  reviewers, translations of added genes or a module `active` flag, and their
  NRPS/PKS/terpene subclasses and RiPP types are not checked. Some rules
  depend on higher levels: RiPPs need precursors from `medium`, compounds need
  a structure at `high`. `Entry.ValidateAt` validates at any level.
* **Consistent application.** In the Python implementation, many quality
  dependent checks never run, because the quality is not passed on (e.g.
  `DomainInfo.from_json` and `ReleaseType.from_json` drop it). go-mibig applies
  the rules consistently, which reports a few more issues in entries of
  quality low or better, e.g. condensation domain subtypes without citations.
* **Release type references** are optional, as in the schema, even though the
  Python code intends to require them. They are always written, and an empty
  list (`"references": []`, as the Python code writes it) is accepted, although
  the schema has `minItems: 1`.
* **Subclusters** of saccharides need `genes`, and `references` above quality
  questionable, as in the Python implementation; the schema makes both
  optional.
* **Coordinates.** `0/0` is accepted (used for "not specified"); the schema's
  minimums of 1 and 2 are not enforced. Start after end is an error.
* **Paths** are required by the schema, but not by the Python implementation
  (`if False and not self.paths: # TODO`), and most entries have none. Not
  enforced.
* **Modification domains** may be of any domain type except those that have
  their own slots in modules (adenylation, AMP-binding, condensation, carrier,
  acyltransferase, ketosynthase). The schema's list is shorter, but the data
  uses ligase, thioesterase, thioreductase, branching and product template
  domains as modification domains.
* **Not checked:** bioactivity names against the bioactivity ontology, and
  cross-checks against GenBank records (gene IDs exist, coordinates within the
  record), which need the sequence data.
* **Molecular formulas** that do not look like `C10H12N2O` (with an optional
  charge) are only warnings.

## Findings in the 5.0rc1 data

[`testdata/data-issues.golden`](../testdata/data-issues.golden) has the
complete list of issues, counted by severity, code and path. The errors are:

| Count | Issue                                                                 |
| ----- | --------------------------------------------------------------------- |
| 95    | duplicate compound evidence                                           |
| 32    | capitalised crosslink types                                           |
| 27    | retired entries without compounds                                     |
| 22    | duplicate biosynthetic classes                                        |
| 13    | modules without a name                                                |
| 13    | condensation subtypes without citations (quality low or better)       |
| 12    | empty RiPP core sequences                                             |
| ...   | a few duplicate list items, missing details and the like              |
