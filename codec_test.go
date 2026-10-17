package mibig

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// roundTrip decodes input into a new T, checks that encoding it gives want,
// and returns the decoded value.
func roundTrip[T any](t *testing.T, input, want string) *T {
	t.Helper()
	var value T
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		t.Fatalf("decoding %s: %v", input, err)
	}
	encoded, err := marshalNoEscape(&value)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if string(encoded) != want {
		t.Errorf("round trip of\n  %s\ngave\n  %s\nwant\n  %s", input, encoded, want)
	}
	return &value
}

func TestCodecKnownFields(t *testing.T) {
	loc := roundTrip[Location](t, `{"to": 20, "from": 10}`, `{"from":10,"to":20}`)
	if loc.From != 10 || loc.To != 20 {
		t.Errorf("got %+v", loc)
	}
}

func TestCodecUnknownFieldsArePreserved(t *testing.T) {
	loc := roundTrip[Location](t,
		`{"extra": [1, 2], "from": 1, "to": 2, "more": null}`,
		`{"from":1,"to":2,"extra":[1,2],"more":null}`)
	if len(loc.Extra.Unknown) != 2 || loc.Extra.Unknown[0].Key != "extra" {
		t.Errorf("unexpected extras %+v", loc.Extra)
	}
	if raw, ok := loc.Extra.Get("more"); !ok || string(raw) != "null" {
		t.Errorf("Extras.Get gave %s, %v", raw, ok)
	}
}

func TestCodecKeysAreCaseSensitive(t *testing.T) {
	loc := roundTrip[Location](t, `{"From": 1, "from": 2, "to": 3}`, `{"from":2,"to":3,"From":1}`)
	if loc.From != 2 {
		t.Errorf("got From %d", loc.From)
	}
}

func TestCodecEmptyValuesArePreserved(t *testing.T) {
	// "subtype": "" and "cyclases": null cannot be represented by the Go
	// fields, so they are recorded and written back in place.
	roundTrip[ThioesteraseDomain](t,
		`{"type":"thioesterase","gene":"a","location":{"from":1,"to":2},"subtype":""}`,
		`{"type":"thioesterase","gene":"a","location":{"from":1,"to":2},"subtype":""}`)
	pks := roundTrip[PKSClass](t,
		`{"class":"PKS","subclass":"Type I","cyclases":null,"ketide_length":0}`,
		`{"class":"PKS","subclass":"Type I","cyclases":null,"ketide_length":0}`)
	if pks.KetideLength == nil || *pks.KetideLength != 0 {
		t.Errorf("ketide length 0 was lost")
	}
	if raw, ok := pks.Extra.Preserved("cyclases"); !ok || string(raw) != "null" || len(pks.Extra.Unknown) != 0 {
		t.Errorf("unexpected extras %+v", pks.Extra)
	}
	// Setting the field takes precedence over the recorded empty value.
	pks.Cyclases = []GeneID{"cycA"}
	encoded, _ := json.Marshal(pks)
	if want := `{"class":"PKS","subclass":"Type I","cyclases":["cycA"],"ketide_length":0}`; string(encoded) != want {
		t.Errorf("got %s, want %s", encoded, want)
	}
}

func TestCodecRequiredFieldsAreWritten(t *testing.T) {
	// Required keys of new values are written even if empty; nil lists as [].
	encoded, err := json.Marshal(Operon{})
	if err != nil || string(encoded) != `{"genes":[],"evidence":[]}` {
		t.Errorf("got %s, %v", encoded, err)
	}
	// Required keys missing from the input stay missing until set.
	operon := roundTrip[Operon](t, `{}`, `{}`)
	operon.Genes = []GeneID{"a"}
	encoded, _ = json.Marshal(operon)
	if string(encoded) != `{"genes":["a"]}` {
		t.Errorf("got %s", encoded)
	}
	// A null in a required list is preserved as null.
	roundTrip[Operon](t, `{"genes":null,"evidence":[]}`, `{"genes":null,"evidence":[]}`)
	// Module "active" is null in many entries.
	roundTrip[PKSTransATModule](t,
		`{"type":"pks-trans-at","name":"1","genes":["a"],"active":null}`,
		`{"type":"pks-trans-at","name":"1","genes":["a"],"active":null}`)
}

func TestCodecAliases(t *testing.T) {
	nrps := roundTrip[NRPSClass](t,
		`{"class":"NRPS","subclass":"Type I","release_type":[{"name":"Hydrolysis","references":[]}]}`,
		`{"class":"NRPS","subclass":"Type I","release_types":[{"name":"Hydrolysis","references":[]}]}`)
	if len(nrps.ReleaseTypes) != 1 {
		t.Errorf("alias not decoded: %+v", nrps)
	}
	roundTrip[TailoringFunction](t,
		`{"function":"Oxidation","references":["pubmed:1"],"dbReference":"mite:MITE0000001"}`,
		`{"function":"Oxidation","references":["pubmed:1"],"db_reference":"mite:MITE0000001"}`)
	roundTrip[NonCanonicalActivity](t,
		`{"evidence":[{"name":"Activity assay"}],"non_elongating":true}`,
		`{"evidence":[{"method":"Activity assay"}],"nonElongating":true}`)
	roundTrip[CALModule](t,
		`{"type":"cal","name":"1","genes":["a"],"active":true,"comments":"note"}`,
		`{"type":"cal","name":"1","genes":["a"],"active":true,"comment":"note"}`)
}

func TestCodecSingleValueForList(t *testing.T) {
	ligase := roundTrip[LigaseDomain](t,
		`{"type":"ligase","gene":"a","location":{"from":1,"to":2},"substrate":"CC","evidence":{"method":"NMR","references":["pubmed:1"]}}`,
		`{"type":"ligase","gene":"a","location":{"from":1,"to":2},"substrates":["CC"],"evidence":[{"method":"NMR","references":["pubmed:1"]}]}`)
	if len(ligase.Substrates) != 1 || len(ligase.Evidence) != 1 {
		t.Errorf("single values not decoded: %+v", ligase)
	}
	roundTrip[MutationPhenotype](t,
		`{"phenotype":"none","references":"pubmed:1"}`,
		`{"phenotype":"none","references":["pubmed:1"]}`)
}

func TestCodecDecodeErrors(t *testing.T) {
	tests := []struct {
		name, input, wantPath, wantMessage string
		into                               any
	}{
		{"duplicate key", `{"from":1,"from":2,"to":3}`, "from", "duplicate key", &Location{}},
		{"alias conflict", `{"class":"NRPS","subclass":"","release_type":[],"release_types":[]}`,
			"release_types", `conflicts with "release_type"`, &NRPSClass{}},
		{"not an object", `[1]`, "", "expected a JSON object", &Location{}},
		{"not a list", `{"genes":"geneA","evidence":[]}`, "genes", "expected a JSON array", &Operon{}},
		{"wrong discriminator", `{"class":"terpene","subclass":""}`, "class", `expected "NRPS"`, &NRPSClass{}},
		{"missing discriminator", `{"subclass":""}`, "", `missing "class" key`, &NRPSClass{}},
		{"wrong domain in a fixed slot",
			`{"type":"nrps-type1","name":"1","genes":[],"active":true,"a_domain":{"type":"condensation","gene":"a","location":{"from":1,"to":2}}}`,
			"a_domain.type", `expected "adenylation"`, &NRPSType1Module{}},
		{"nested path", `{"classes":[{"class":"PKS","subclass":"","cyclases":[1]}]}`,
			"classes[0].cyclases[0]", "cannot unmarshal number", &Biosynthesis{}},
		{"wrong scalar type", `{"from":"1","to":2}`, "from", "cannot unmarshal string", &Location{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tt.input), tt.into)
			var de *DecodeError
			if !errors.As(err, &de) {
				t.Fatalf("expected a *DecodeError, got %v", err)
			}
			if de.Path != tt.wantPath || !strings.Contains(de.Err.Error(), tt.wantMessage) {
				t.Errorf("got path %q, message %q; want %q, %q", de.Path, de.Err, tt.wantPath, tt.wantMessage)
			}
		})
	}
}

func TestCodecDiscriminatorAlternatives(t *testing.T) {
	// The schema calls the PKS class "pks", the data "PKS".
	roundTrip[PKSClass](t, `{"class":"pks","subclass":"Type I"}`, `{"class":"PKS","subclass":"Type I"}`)
}

func TestUnionLists(t *testing.T) {
	input := `{"classes":[` +
		`{"class":"terpene","subclass":"Diterpene"},` +
		`{"class":"alien","subclass":"x","weird":[1]},` +
		`{"class":7},` +
		`null` +
		`],"modules":[{"type":"mystery","name":"7","genes":["a"],"active":true}]}`
	b := roundTrip[Biosynthesis](t, input,
		`{"classes":[{"class":"terpene","subclass":"Diterpene"},{"class":"alien","subclass":"x","weird":[1]},{"class":7},null],`+
			`"modules":[{"type":"mystery","name":"7","genes":["a"],"active":true}]}`)

	if _, ok := b.Classes[0].(*TerpeneClass); !ok {
		t.Errorf("classes[0] is %T", b.Classes[0])
	}
	if unknown, ok := b.Classes[1].(*UnknownClass); !ok || unknown.ClassName() != "alien" {
		t.Errorf("classes[1] is %#v", b.Classes[1])
	}
	if unknown, ok := b.Classes[2].(*UnknownClass); !ok || unknown.ClassName() != "" {
		t.Errorf("classes[2] is %#v", b.Classes[2])
	}
	if b.Classes[3] != nil {
		t.Errorf("classes[3] is %#v", b.Classes[3])
	}
	unknown, ok := b.Modules[0].(*UnknownModule)
	if !ok || unknown.ModuleType() != "mystery" || unknown.Base().Name != "7" {
		t.Errorf("modules[0] is %#v", b.Modules[0])
	}
}

func TestInterfaceField(t *testing.T) {
	cal := roundTrip[CALModule](t,
		`{"type":"cal","name":"1","genes":["a"],"active":true,"cal":{"type":"ligase","gene":"a","location":{"from":1,"to":2}}}`,
		`{"type":"cal","name":"1","genes":["a"],"active":true,"cal":{"type":"ligase","gene":"a","location":{"from":1,"to":2}}}`)
	if _, ok := cal.CAL.(*LigaseDomain); !ok {
		t.Errorf("cal domain is %T", cal.CAL)
	}
}

func TestUnknownVariantsCreatedInGo(t *testing.T) {
	encoded, err := json.Marshal(Biosynthesis{
		Classes: Classes{&UnknownClass{Name: "lipid"}},
		Modules: Modules{&UnknownModule{Type: "mystery"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"classes":[{"class":"lipid"}],"modules":[{"type":"mystery"}]}`; string(encoded) != want {
		t.Errorf("got %s, want %s", encoded, want)
	}
}

func TestTypedNilInUnionList(t *testing.T) {
	classes := Classes{(*NRPSClass)(nil), nil}
	encoded, err := json.Marshal(classes)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "[null,null]" {
		t.Errorf("got %s", encoded)
	}
}

func TestBioactivityName(t *testing.T) {
	b := roundTrip[Bioactivity](t,
		`{"name":"antibacterial","observed":true,"references":[]}`,
		`{"name":"antibacterial","observed":true,"references":[]}`)
	if b.Name.Activity != "antibacterial" || b.Name.Legacy {
		t.Errorf("got %+v", b.Name)
	}
	b = roundTrip[Bioactivity](t,
		`{"name":{"activity":"antiviral"},"observed":false,"references":[]}`,
		`{"name":{"activity":"antiviral"},"observed":false,"references":[]}`)
	if b.Name.String() != "antiviral" || !b.Name.Legacy {
		t.Errorf("got %+v", b.Name)
	}
	roundTrip[Bioactivity](t,
		`{"name":{"activity":"antiviral","comment":"anti-HIV"},"observed":true,"references":[]}`,
		`{"name":{"activity":"antiviral","comment":"anti-HIV"},"observed":true,"references":[]}`)

	// Unknown keys and empty values of the legacy form are preserved too.
	b = roundTrip[Bioactivity](t,
		`{"name":{"activity":"x","comment":"","colour":"red"},"observed":true,"references":[]}`,
		`{"name":{"activity":"x","comment":"","colour":"red"},"observed":true,"references":[]}`)
	if _, ok := b.Name.Extra.Preserved("comment"); !ok || len(b.Name.Extra.Unknown) != 1 {
		t.Errorf("got extras %+v", b.Name.Extra)
	}
}

func TestCodecRejectsNullListElements(t *testing.T) {
	for _, input := range []string{
		`{"genes":["a",null],"evidence":[]}`,
		`{"genes":[],"evidence":[null]}`,
	} {
		var operon Operon
		err := json.Unmarshal([]byte(input), &operon)
		var de *DecodeError
		if !errors.As(err, &de) || !strings.Contains(de.Err.Error(), "null is not allowed") {
			t.Errorf("%s: got %v", input, err)
		}
	}
	var p RiPPPrecursor
	if err := json.Unmarshal([]byte(`{"gene":"a","core_sequence":["A",null]}`), &p); err == nil {
		t.Error("expected an error for null in a core sequence list")
	}
	// Raw JSON lists keep their nulls.
	roundTrip[Compound](t, `{"name":"x","evidence":[],"targets":[null,1]}`, `{"name":"x","evidence":[],"targets":[null,1]}`)
}

func TestCodecDuplicateDiscriminator(t *testing.T) {
	var c NRPSClass
	if err := json.Unmarshal([]byte(`{"class":"NRPS","class":"NRPS","subclass":""}`), &c); err == nil {
		t.Error("expected an error for a duplicate discriminator")
	}
	for _, input := range []string{
		`{"class":"NRPS","class":"future"}`,
		`{"class":"future","class":"NRPS"}`,
		`{"class":"future","class":"future"}`,
	} {
		var classes Classes
		if err := json.Unmarshal([]byte("["+input+"]"), &classes); err == nil {
			t.Errorf("%s: expected an error for a duplicate discriminator", input)
		}
	}
}

func TestCodecReplacesExistingValues(t *testing.T) {
	var loc Location
	if err := json.Unmarshal([]byte(`{"from":null,"to":2,"x":1}`), &loc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"from":1,"to":3}`), &loc); err != nil {
		t.Fatal(err)
	}
	if !loc.Extra.IsZero() || loc.From != 1 || loc.To != 3 {
		t.Errorf("decoding did not replace the value: %+v", loc)
	}
}

func TestCodecQuotesOddKeysInPaths(t *testing.T) {
	var loc Location
	err := json.Unmarshal([]byte(`{"from":1,"to":2,"a.b":1,"a.b":2}`), &loc)
	if err != nil {
		t.Fatalf("unknown duplicate keys are kept: %v", err)
	}
	var c NRPSClass
	err = json.Unmarshal([]byte(`{"class":"NRPS","subclass":"","thioesterases":[{"type":"thioesterase","gene":"a","location":{"from":"x"}}]}`), &c)
	var de *DecodeError
	if !errors.As(err, &de) || de.Path != "thioesterases[0].location.from" {
		t.Errorf("got %v", err)
	}
}

func TestMarshalErrorPaths(t *testing.T) {
	e := validEntry()
	e.Compounds = append(e.Compounds, Compound{Name: "y", Mass: "heavy"})
	_, err := Marshal(e)
	var de *DecodeError
	if !errors.As(err, &de) || de.Path != "compounds[1].mass" {
		t.Errorf("got %v", err)
	}
}

func TestNumbersKeepTheirSpelling(t *testing.T) {
	c := roundTrip[Compound](t, `{"name":"x","evidence":[],"mass":500.0}`, `{"name":"x","evidence":[],"mass":500.0}`)
	if mass, err := c.Mass.Float64(); err != nil || mass != 500 {
		t.Errorf("got %v, %v", mass, err)
	}
	roundTrip[Compound](t, `{"name":"x","evidence":[],"mass":1e-7}`, `{"name":"x","evidence":[],"mass":1e-7}`)
	var compound Compound
	if err := json.Unmarshal([]byte(`{"name":"x","evidence":[],"mass":"500"}`), &compound); err == nil {
		t.Error("expected an error for a mass given as a string")
	}
}

func TestParseNeedsAnObject(t *testing.T) {
	for _, input := range []string{"null", "[]", "1", ""} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("Parse(%q) did not fail", input)
		}
	}
}

func TestCoreSequence(t *testing.T) {
	p := roundTrip[RiPPPrecursor](t, `{"gene":"a","core_sequence":"ITS"}`, `{"gene":"a","core_sequence":"ITS"}`)
	if len(p.CoreSequence.Sequences) != 1 || p.CoreSequence.Legacy {
		t.Errorf("got %+v", p.CoreSequence)
	}
	p = roundTrip[RiPPPrecursor](t, `{"gene":"a","core_sequence":["ITS"]}`, `{"gene":"a","core_sequence":["ITS"]}`)
	if !p.CoreSequence.Legacy {
		t.Errorf("got %+v", p.CoreSequence)
	}
	roundTrip[RiPPPrecursor](t, `{"gene":"a","core_sequence":["A","B"]}`, `{"gene":"a","core_sequence":["A","B"]}`)
	roundTrip[RiPPPrecursor](t, `{"gene":"a","core_sequence":""}`, `{"gene":"a","core_sequence":""}`)
}

func TestDate(t *testing.T) {
	r := roundTrip[Release](t, `{"version":"next","entries":[],"date":null}`, `{"version":"next","entries":[],"date":null}`)
	if r.Date != "" {
		t.Errorf("got %q", r.Date)
	}
	r = roundTrip[Release](t, `{"version":"1","entries":[],"date":"2024-11-15"}`, `{"version":"1","entries":[],"date":"2024-11-15"}`)
	if tm, err := r.Date.Time(); err != nil || tm.Year() != 2024 {
		t.Errorf("got %v, %v", tm, err)
	}
}

// TestAllTypesUseTheCodec makes sure that every struct reachable from Entry
// that can hold unknown keys has the codec's JSON methods; without them,
// encoding/json would silently drop the unknown keys.
func TestAllTypesUseTheCodec(t *testing.T) {
	var roots []any
	roots = append(roots, Entry{})
	for _, newClass := range classUnion.variants {
		roots = append(roots, newClass())
	}
	for _, newModule := range moduleUnion.variants {
		roots = append(roots, newModule())
	}
	for _, newDomain := range domainUnion.variants {
		roots = append(roots, newDomain())
	}
	for _, missing := range typesWithoutCodec(roots...) {
		t.Errorf("%s has an Extras field, but no MarshalJSON/UnmarshalJSON methods", missing)
	}
	visitStructTypes(func(st reflect.Type, info *structInfo) {
		for _, fi := range info.fields {
			if fi.nonEmpty && (!fi.omitZero || st.FieldByIndex(fi.index).Type.Kind() != reflect.Slice) {
				t.Errorf("%s.%s: nonempty is only for omitzero lists", st.Name(), fi.name)
			}
		}
	}, roots...)
}

// typesWithoutCodec returns the struct types reachable from the given values
// that have an Extras field, but do not implement json.Marshaler and
// json.Unmarshaler.
func typesWithoutCodec(roots ...any) []string {
	var missing []string
	visitStructTypes(func(t reflect.Type, info *structInfo) {
		if info.extras != nil && (!t.Implements(marshalerType) || !reflect.PointerTo(t).Implements(unmarshalerType)) {
			missing = append(missing, t.Name())
		}
	}, roots...)
	return missing
}

// visitStructs calls visit for the codec metadata of every struct type
// reachable from the given values.
func visitStructs(visit func(info *structInfo), roots ...any) {
	visitStructTypes(func(_ reflect.Type, info *structInfo) { visit(info) }, roots...)
}

func visitStructTypes(visit func(t reflect.Type, info *structInfo), roots ...any) {
	seen := map[reflect.Type]bool{}
	var walkType func(t reflect.Type)
	walkType = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		info := getStructInfo(t)
		visit(t, info)
		for _, fi := range info.fields {
			walkType(t.FieldByIndex(fi.index).Type)
		}
	}
	for _, root := range roots {
		walkType(reflect.TypeOf(root))
	}
}

func TestCodecDiscardPreserved(t *testing.T) {
	encode := func(v any) string {
		t.Helper()
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}

	// A preserved null under an alias comes back whenever the field is
	// cleared, until it is discarded; then the optional key is deleted.
	m := roundTrip[OtherModule](t,
		`{"type":"other","name":"m","genes":[],"active":true,"comments":null}`,
		`{"type":"other","name":"m","genes":[],"active":true,"comments":null}`)
	if raw, ok := m.Extra.Preserved("comment"); !ok || string(raw) != "null" {
		t.Errorf("Preserved gave %s, %v", raw, ok)
	}
	m.Comment = "x"
	if got, want := encode(m), `{"type":"other","name":"m","genes":[],"active":true,"comment":"x"}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	m.Comment = ""
	if got, want := encode(m), `{"type":"other","name":"m","genes":[],"active":true,"comments":null}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	m.Extra.Discard("comment")
	if got, want := encode(m), `{"type":"other","name":"m","genes":[],"active":true}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, ok := m.Extra.Preserved("comments"); ok {
		t.Errorf("preserved value was not discarded")
	}

	// Aliases name the field, whichever spelling was decoded.
	m = roundTrip[OtherModule](t,
		`{"type":"other","name":"m","genes":[],"active":true,"comment":null}`,
		`{"type":"other","name":"m","genes":[],"active":true,"comment":null}`)
	if raw, ok := m.Extra.Preserved("comments"); !ok || string(raw) != "null" {
		t.Errorf("Preserved through alias gave %s, %v", raw, ok)
	}
	m.Extra.Discard("comments")
	if got, want := encode(m), `{"type":"other","name":"m","genes":[],"active":true}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// A missing key that is always written stays missing until discarded;
	// then the zero value is written.
	loc := roundTrip[Location](t, `{"from":1}`, `{"from":1}`)
	if raw, ok := loc.Extra.Preserved("to"); !ok || raw != nil {
		t.Errorf("Preserved gave %s, %v", raw, ok)
	}
	original := *loc
	loc.Extra.Discard("to")
	if got, want := encode(loc), `{"from":1,"to":0}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	// Discarding does not affect copies.
	if got, want := encode(original), `{"from":1}`; got != want {
		t.Errorf("copy: got %s, want %s", got, want)
	}
	both := roundTrip[Location](t, `{}`, `{}`)
	copied := *both
	both.Extra.Discard("from")
	if got, want := encode(both), `{"from":0}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got, want := encode(copied), `{}`; got != want {
		t.Errorf("copy: got %s, want %s", got, want)
	}

	// DiscardAll keeps unknown keys.
	pks := roundTrip[PKSClass](t,
		`{"class":"PKS","subclass":"","cyclases":null,"colour":"red"}`,
		`{"class":"PKS","subclass":"","cyclases":null,"colour":"red"}`)
	pks.Extra.DiscardAll()
	if got, want := encode(pks), `{"class":"PKS","subclass":"","colour":"red"}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
