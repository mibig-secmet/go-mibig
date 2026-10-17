package mibig_test

import (
	"errors"
	"slices"
	"testing"

	mibig "go.kblin.dev/go-mibig"
)

func TestVocabularies(t *testing.T) {
	entry, err := mibig.ReadFile("testdata/entries/BGC0000370.json")
	if err != nil {
		t.Fatal(err)
	}
	vocabs := entry.Vocabularies()
	for path, want := range map[string]string{
		"quality":                    "high",
		"status":                     "active",
		"completeness":               "complete",
		"loci[0].evidence[0].method": "Knock-out studies",
	} {
		vocab, ok := vocabs[path]
		if !ok {
			t.Errorf("no vocabulary for %s", path)
			continue
		}
		if !slices.Contains(vocab, want) {
			t.Errorf("vocabulary of %s = %q, missing %q", path, vocab, want)
		}
	}
	// the result is a copy
	vocabs["quality"][0] = "changed"
	if entry.Vocabularies()["quality"][0] == "changed" {
		t.Error("Vocabularies returned the internal slice")
	}
}

func TestVocabulariesOfEmptyFields(t *testing.T) {
	entry := &mibig.Entry{
		Loci: []mibig.Locus{{Evidence: []mibig.Evidence{{}}}},
		Biosynthesis: mibig.Biosynthesis{
			Classes: mibig.Classes{mibig.NewClass(mibig.ClassNRPS)},
		},
	}
	vocabs := entry.Vocabularies()
	for _, path := range []string{"loci[0].evidence[0].method", "biosynthesis.classes[0].subclass"} {
		if len(vocabs[path]) == 0 {
			t.Errorf("no vocabulary for empty field %s", path)
		}
	}
}

func TestVariants(t *testing.T) {
	names := mibig.ClassNames()
	if !slices.Contains(names, mibig.ClassPKS) || slices.Contains(names, "pks") {
		t.Errorf("ClassNames() = %q, want PKS without alias pks", names)
	}
	if !slices.IsSorted(names) {
		t.Errorf("ClassNames() not sorted: %q", names)
	}
	for _, name := range names {
		if c := mibig.NewClass(name); c == nil || c.ClassName() != name {
			t.Errorf("NewClass(%q) = %#v", name, c)
		}
	}
	for _, mt := range mibig.ModuleTypes() {
		if m := mibig.NewModule(mt); m == nil || m.ModuleType() != mt {
			t.Errorf("NewModule(%q) = %#v", mt, m)
		}
	}
	for _, dt := range mibig.DomainTypes() {
		if d := mibig.NewDomain(dt); d == nil || d.DomainType() != dt {
			t.Errorf("NewDomain(%q) = %#v", dt, d)
		}
	}
	if len(mibig.DomainTypes()) != 21 || len(mibig.ModuleTypes()) != 9 {
		t.Errorf("got %d domain types, %d module types", len(mibig.DomainTypes()), len(mibig.ModuleTypes()))
	}
	if mibig.NewClass("nope") != nil || mibig.NewModule("nope") != nil || mibig.NewDomain("nope") != nil {
		t.Error("unknown variants should be nil")
	}
}

func TestUnmarshalVariants(t *testing.T) {
	class, err := mibig.UnmarshalClass([]byte(`{"class": "pks", "subclass": "Type I"}`))
	if err != nil {
		t.Fatal(err)
	}
	if pks, ok := class.(*mibig.PKSClass); !ok || pks.Subclass != "Type I" {
		t.Errorf("UnmarshalClass = %#v", class)
	}
	class, err = mibig.UnmarshalClass([]byte(`{"class": "alien"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := class.(*mibig.UnknownClass); !ok {
		t.Errorf("unknown class decoded as %T", class)
	}

	domain, err := mibig.UnmarshalDomain([]byte(`{"type": "adenylation"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := domain.(*mibig.AdenylationDomain); !ok {
		t.Errorf("UnmarshalDomain = %T", domain)
	}
	if _, err := mibig.UnmarshalModule([]byte(`{"type": "cal", "name": 5}`)); err == nil {
		t.Error("expected an error for a wrongly typed field")
	}
	var de *mibig.DecodeError
	if _, err := mibig.UnmarshalModule([]byte(`{"type": `)); !errors.As(err, &de) {
		t.Errorf("invalid JSON: got %v, want a DecodeError", err)
	}
}
