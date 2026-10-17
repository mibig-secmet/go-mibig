# Test data

* `entries/`: 21 entries from the MIBiG 5.0rc1 release, chosen to cover
  different classes, modules, domains, gene annotations, legacy formats and
  leftovers of older conversions. The `.issues` files hold their expected
  validation issues; update them with `go test -run TestEntryFixtures -update`.
  MIBiG data is available under the
  [Creative Commons Attribution 4.0 license](https://creativecommons.org/licenses/by/4.0/).
* `schema-examples/`: the usable examples of the schema repository
  (`schema/examples`). Empty placeholder examples were left out, and
  `mibig/biosynthesis/saccharide/acarbose.json` got a `"class"` key, which the
  original lacks.
* `schema-complete.json`: a made-up entry that uses every class, module and
  domain type and all optional fields, including the parts of the schema the
  data does not use yet. It must validate cleanly at quality high and be
  written back byte for byte.
* `format/`: an entry with unusual characters (`input.json`) and the output of
  Python's `json.dumps(entry, indent=2)` for it (`python_output.json`).
* `data-issues.golden`: the validation issues of the whole MIBiG 5.0rc1
  release, counted by severity, code and path. Used when `MIBIG_DATA_DIR` is
  set.
