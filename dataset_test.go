package mibig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// The tests in this file run against a full MIBiG data release, if the
// MIBIG_DATA_DIR environment variable points to a directory of entry files.

func dataFiles(t *testing.T) []string {
	t.Helper()
	dir := os.Getenv("MIBIG_DATA_DIR")
	if dir == "" {
		t.Skip("MIBIG_DATA_DIR not set")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no JSON files in %s", dir)
	}
	sort.Strings(files)
	return files
}

// TestDataRoundTrip checks that every entry decodes, and that encoding it
// again gives the same JSON document (ignoring key order and formatting),
// and that a second encoding is byte-identical to the first.
func TestDataRoundTrip(t *testing.T) {
	for _, file := range dataFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := Parse(data)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(file), err)
			continue
		}
		encoded, err := Marshal(entry)
		if err != nil {
			t.Errorf("%s: marshal: %v", filepath.Base(file), err)
			continue
		}
		if diff := jsonDiff(data, encoded); diff != "" {
			t.Errorf("%s: round trip changed the document:\n%s", filepath.Base(file), diff)
			continue
		}
		again, err := Parse(encoded)
		if err != nil {
			t.Errorf("%s: parsing the re-encoded entry: %v", filepath.Base(file), err)
			continue
		}
		encodedAgain, err := Marshal(again)
		if err != nil {
			t.Errorf("%s: marshal: %v", filepath.Base(file), err)
			continue
		}
		if !bytes.Equal(encoded, encodedAgain) {
			t.Errorf("%s: second encoding differs from the first", filepath.Base(file))
		}
	}
}

// jsonDiff compares two JSON documents, ignoring key order and formatting,
// and describes the first few differences.
func jsonDiff(a, b []byte) string {
	return jsonDiffWith(a, b, nil)
}

// jsonDiffNormalised is jsonDiff, but accepts the normalisations the codec
// makes on purpose: alias keys renamed to their canonical name, and single
// values turned into lists.
func jsonDiffNormalised(a, b []byte) string {
	return jsonDiffWith(a, b, codecAliases())
}

// codecAliases maps the alias keys of all schema types to their canonical
// names.
func codecAliases() map[string]string {
	aliases := map[string]string{}
	var roots []any
	roots = append(roots, Entry{})
	for _, newModule := range moduleUnion.variants {
		roots = append(roots, newModule())
	}
	for _, newClass := range classUnion.variants {
		roots = append(roots, newClass())
	}
	for _, newDomain := range domainUnion.variants {
		roots = append(roots, newDomain())
	}
	visitStructs(func(info *structInfo) {
		for _, fi := range info.fields {
			for _, alias := range fi.aliases {
				aliases[alias] = fi.name
			}
		}
	}, roots...)
	return aliases
}

func jsonDiffWith(a, b []byte, aliases map[string]string) string {
	var va, vb any
	if err := unmarshalNumbers(a, &va); err != nil {
		return "first document: " + err.Error()
	}
	if err := unmarshalNumbers(b, &vb); err != nil {
		return "second document: " + err.Error()
	}
	var diffs []string
	c := &jsonComparer{aliases: aliases, diffs: &diffs}
	c.compare("", va, vb)
	if len(diffs) == 0 {
		return ""
	}
	if len(diffs) > 10 {
		diffs = append(diffs[:10], fmt.Sprintf("... and %d more", len(diffs)-10))
	}
	var buf bytes.Buffer
	for _, d := range diffs {
		buf.WriteString("  " + d + "\n")
	}
	return buf.String()
}

func unmarshalNumbers(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

type jsonComparer struct {
	aliases map[string]string // nil for an exact comparison
	diffs   *[]string
}

func (c *jsonComparer) compare(path string, a, b any) {
	diffs := c.diffs
	if c.aliases != nil {
		// a single value may have become a list
		if list, ok := b.([]any); ok && len(list) == 1 {
			if _, isList := a.([]any); !isList {
				b = list[0]
			}
		}
	}
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: %s became %s", path, short(a), short(b)))
			return
		}
		keys := make([]string, 0, len(av)+len(bv))
		for k := range av {
			keys = append(keys, k)
		}
		for k := range bv {
			if _, ok := av[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			x, inA := av[k]
			y, inB := bv[k]
			if canonical, isAlias := c.aliases[k]; isAlias && inA && !inB {
				if _, renamed := av[canonical]; !renamed {
					y, inB = bv[canonical]
				}
			}
			if canonical, isAlias := c.aliases[k]; isAlias && !inA && inB {
				if _, ok := bv[canonical]; ok {
					continue // reported under the canonical name
				}
			}
			if _, gainedByAlias := c.renamedTo(av, bv, k); gainedByAlias {
				continue
			}
			switch {
			case !inB:
				*diffs = append(*diffs, fmt.Sprintf("%s: lost %s", join(path, k), short(x)))
			case !inA:
				*diffs = append(*diffs, fmt.Sprintf("%s: gained %s", join(path, k), short(y)))
			default:
				c.compare(join(path, k), x, y)
			}
		}
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			*diffs = append(*diffs, fmt.Sprintf("%s: %s became %s", path, short(a), short(b)))
			return
		}
		for i := range av {
			c.compare(index(path, i), av[i], bv[i])
		}
	case json.Number:
		bv, ok := b.(json.Number)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: %s became %s", path, short(a), short(b)))
			return
		}
		if c.aliases == nil {
			// exact comparison: numbers must be written the same way
			if av != bv {
				*diffs = append(*diffs, fmt.Sprintf("%s: %s became %s", path, av, bv))
			}
			return
		}
		fa, errA := av.Float64()
		fb, errB := bv.Float64()
		if errA != nil || errB != nil || fa != fb {
			*diffs = append(*diffs, fmt.Sprintf("%s: %s became %s", path, av, bv))
		}
	default:
		if !reflect.DeepEqual(a, b) {
			*diffs = append(*diffs, fmt.Sprintf("%s: %s became %s", path, short(a), short(b)))
		}
	}
}

// renamedTo reports whether the canonical key k of b was written for an
// alias key of a.
func (c *jsonComparer) renamedTo(a, b map[string]any, k string) (string, bool) {
	if _, inA := a[k]; inA {
		return "", false
	}
	if _, inB := b[k]; !inB {
		return "", false
	}
	for alias, canonical := range c.aliases {
		if canonical == k {
			if _, ok := a[alias]; ok {
				return alias, true
			}
		}
	}
	return "", false
}

func short(v any) string {
	data, _ := json.Marshal(v)
	if len(data) > 60 {
		return string(data[:57]) + "..."
	}
	return string(data)
}

var indexPattern = regexp.MustCompile(`\[\d+\]`)

// issueSummary counts validation issues by severity, code and path, with
// list indices removed, e.g. "error invalid-value compounds[].evidence[].method".
func issueSummary(t *testing.T) map[string]int {
	t.Helper()
	summary := map[string]int{}
	for _, file := range dataFiles(t) {
		entry, err := ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, issue := range entry.Validate() {
			key := fmt.Sprintf("%s %s %s", issue.Severity, issue.Code, indexPattern.ReplaceAllString(issue.Path, "[]"))
			summary[key]++
		}
	}
	return summary
}

// TestDataValidateBaseline compares the validation issues of the whole data
// release, counted by severity, code and path, against a reviewed baseline.
// The baseline belongs to MIBiG 5.0rc1; regenerate it with -update for other
// releases or after changing validation rules.
func TestDataValidateBaseline(t *testing.T) {
	summary := issueSummary(t)
	keys := make([]string, 0, len(summary))
	for k := range summary {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&buf, "%6d %s\n", summary[k], k)
	}
	checkGolden(t, "testdata/data-issues.golden", buf.String())
}
