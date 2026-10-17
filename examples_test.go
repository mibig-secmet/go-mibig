package mibig

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// schemaExampleTypes maps the example directories of the schema repository
// (copied to testdata/schema-examples) to the Go types they decode into.
var schemaExampleTypes = map[string]func() any{
	"common/changelog":                        func() any { return &Changelog{} },
	"common/citation":                         func() any { return new(Citation) },
	"common/gene_id":                          func() any { return new(GeneID) },
	"common/location":                         func() any { return &Location{} },
	"common/smiles":                           func() any { return new(SMILES) },
	"mibig/biosynthesis/common/monomer":       func() any { return &Monomer{} },
	"mibig/biosynthesis/common/non_canonical": func() any { return &NonCanonicalActivity{} },
	"mibig/biosynthesis/common/release_type":  func() any { return &ReleaseType{} },
	"mibig/biosynthesis/domains":              func() any { return &Domains{} },
	"mibig/biosynthesis/nrps":                 func() any { return &Classes{} },
	"mibig/biosynthesis/other":                func() any { return &Classes{} },
	"mibig/biosynthesis/saccharide":           func() any { return &Classes{} },
	"mibig/biosynthesis/terpene":              func() any { return &Classes{} },
	"mibig/common/substrate_evidence":         func() any { return &Evidence{} },
	"mibig/entry":                             func() any { return &Entry{} },
	"mibig/taxonomy":                          func() any { return &Taxonomy{} },
}

// TestSchemaExamples decodes the examples of the schema repository and checks
// that they validate at quality high, without unknown keys, empty values or
// missing required keys.
func TestSchemaExamples(t *testing.T) {
	files, err := filepath.Glob("testdata/schema-examples/*/*/*.json")
	if err != nil {
		t.Fatal(err)
	}
	more, _ := filepath.Glob("testdata/schema-examples/*/*/*/*.json")
	files = append(files, more...)
	more, _ = filepath.Glob("testdata/schema-examples/*/*/*/*/*.json")
	files = append(files, more...)

	checked := 0
	for _, file := range files {
		rel, _ := filepath.Rel("testdata/schema-examples", file)
		newValue := exampleType(filepath.ToSlash(filepath.Dir(rel)))
		if newValue == nil {
			t.Errorf("%s: no Go type registered for this example", rel)
			continue
		}
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			value := newValue()
			// Classes and domains are tested as one element lists.
			switch value.(type) {
			case *Domains, *Classes:
				data = append(append([]byte("["), data...), ']')
			}
			if err := json.Unmarshal(data, value); err != nil {
				t.Fatal(err)
			}
			v := &validator{quality: QualityHigh}
			validateValue(v, value)
			for _, issue := range v.issues {
				t.Errorf("unexpected issue: %s", issue)
			}
		})
		checked++
	}
	if checked < 30 {
		t.Errorf("only %d examples checked", checked)
	}
}

func exampleType(dir string) func() any {
	for prefix := dir; prefix != "."; prefix = filepath.ToSlash(filepath.Dir(prefix)) {
		if newValue, ok := schemaExampleTypes[prefix]; ok {
			return newValue
		}
	}
	return nil
}

// validateValue runs the validation of any schema type, including the checks
// of its Extras.
func validateValue(v *validator, value any) {
	defer validateExtras(v, value)
	switch value := value.(type) {
	case *Entry:
		value.validate(v)
	case *Changelog:
		value.validate(v, "")
	case *Citation:
		value.validate(v, "")
	case *GeneID:
		value.validate(v, "")
	case *Location:
		value.validate(v, "")
	case *SMILES:
		value.validate(v, "")
	case *Monomer:
		value.validate(v, "")
	case *NonCanonicalActivity:
		value.validate(v, "")
	case *ReleaseType:
		value.validate(v, "")
	case *Domains:
		value.validate(v, "", nil)
	case *Classes:
		value.validate(v, "")
	case *Evidence:
		value.validate(v, "", substrateEvidenceMethods)
	case *Taxonomy:
		value.validate(v, "")
	default:
		panic(fmt.Sprintf("no validation for %T", value))
	}
}

// TestEntryFixtures checks the validation issues of real entries against
// golden files. Run with -update to regenerate them.
func TestEntryFixtures(t *testing.T) {
	files, _ := filepath.Glob("testdata/entries/*.json")
	if len(files) == 0 {
		t.Fatal("no fixtures")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			entry, err := ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, issue := range entry.Validate() {
				lines = append(lines, issue.String())
			}
			got := strings.Join(lines, "\n") + "\n"
			goldenFile := strings.TrimSuffix(file, ".json") + ".issues"
			checkGolden(t, goldenFile, got)
		})
	}
}

func checkGolden(t *testing.T, goldenFile, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(goldenFile, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("issues differ from %s (run with -update to accept):\n%s", goldenFile, lineDiff(want, []byte(got)))
	}
}

// TestSchemaComplete checks an entry that uses every class, module and domain
// type and all optional fields of the schema: it must decode without unknown
// keys, validate cleanly at quality high and be written back byte for byte.
func TestSchemaComplete(t *testing.T) {
	data, err := os.ReadFile("testdata/schema-complete.json")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range entry.Validate() {
		t.Errorf("unexpected issue: %s", issue)
	}
	walk(reflect.ValueOf(entry), "", func(rv reflect.Value, path string) {
		if rv.Kind() != reflect.Struct {
			return
		}
		if info := getStructInfo(rv.Type()); info.extras != nil && !rv.FieldByIndex(info.extras).Interface().(Extras).IsZero() {
			t.Errorf("%s: unexpected extras %v", path, rv.FieldByIndex(info.extras).Interface())
		}
	})

	seenClasses := map[ClassName]bool{}
	for _, class := range entry.Biosynthesis.Classes {
		seenClasses[class.ClassName()] = true
	}
	seenModules := map[ModuleType]bool{}
	seenDomains := map[DomainType]bool{}
	walk(reflect.ValueOf(entry), "", func(rv reflect.Value, _ string) {
		if rv.CanAddr() {
			switch value := rv.Addr().Interface().(type) {
			case Module:
				seenModules[value.ModuleType()] = true
			case Domain:
				seenDomains[value.DomainType()] = true
			}
		}
	})
	if len(seenClasses) != len(classUnion.variants)-1 { // "pks" and "PKS"
		t.Errorf("only %d class types used", len(seenClasses))
	}
	if len(seenModules) != len(moduleUnion.variants) {
		t.Errorf("only %d module types used", len(seenModules))
	}
	if len(seenDomains) != len(domainUnion.variants) {
		t.Errorf("only %d of %d domain types used", len(seenDomains), len(domainUnion.variants))
	}

	encoded, err := Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, data) {
		t.Errorf("not written back byte for byte:\n%s", lineDiff(data, encoded))
	}
}
