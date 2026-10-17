package mibig

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// validEntry returns a small entry that is valid at quality high.
func validEntry() *Entry {
	refs := []Citation{"pubmed:12345"}
	return &Entry{
		Accession: "BGC0000001",
		Version:   1,
		Changelog: Changelog{Releases: []Release{{
			Version: "4.0",
			Date:    "2024-11-15",
			Entries: []ReleaseEntry{{
				Contributors: []SubmitterID{"AAAAAAAAAAAAAAAAAAAAAAAA"},
				Reviewers:    []SubmitterID{"BBBBBBBBBBBBBBBBBBBBBBBB"},
				Date:         "2024-07-01",
				Comment:      "Submitted",
			}},
		}}},
		Quality:      QualityHigh,
		Status:       StatusActive,
		Completeness: CompletenessComplete,
		Loci: []Locus{{
			Accession: "AB000001.1",
			Location:  Location{From: 1, To: 20000},
			Evidence:  []Evidence{{Method: "Knock-out studies", References: refs}},
		}},
		Biosynthesis: Biosynthesis{
			Classes: Classes{&NRPSClass{Subclass: "Type I"}},
			Modules: Modules{&NRPSType1Module{
				ModuleBase: ModuleBase{Name: "1", Genes: []GeneID{"nrpA"}, Active: Bool(true)},
				ADomain: &AdenylationDomain{
					DomainBase: DomainBase{Gene: "nrpA", Location: Location{From: 10, To: 400}},
					Substrates: []AdenylationSubstrate{{Name: "alanine", Structure: "NC(C)C(=O)O", Proteinogenic: Bool(true)}},
					Evidence:   []Evidence{{Method: "Structure-based inference", References: refs}},
				},
			}},
		},
		Compounds: []Compound{{
			Name:      "examplin",
			Evidence:  []CompoundEvidence{{Method: "NMR", References: refs}},
			Structure: "CC(N)C(=O)O",
			Bioactivities: []Bioactivity{{
				Name: BioactivityName{Activity: "antibacterial"}, Observed: Bool(true), References: refs,
			}},
		}},
		Taxonomy: Taxonomy{Name: "Streptomyces coelicolor A3(2)", NCBITaxID: 100226},
	}
}

func TestValidEntry(t *testing.T) {
	if issues := validEntry().Validate(); len(issues) > 0 {
		t.Errorf("unexpected issues: %v", issues)
	}
}

func TestInvalidJSONNumbers(t *testing.T) {
	fields := []struct {
		path string
		set  func(*Entry, json.Number)
	}{
		{"compounds[0].mass", func(e *Entry, n json.Number) { e.Compounds[0].Mass = n }},
		{"compounds[0].bioactivities[0].assays[0].measurement.concentration", func(e *Entry, n json.Number) {
			e.Compounds[0].Bioactivities[0].Assays[0].Measurement.Concentration = n
		}},
		{"compounds[0].bioactivities[0].assays[0].measurement.error", func(e *Entry, n json.Number) {
			e.Compounds[0].Bioactivities[0].Assays[0].Measurement.Error = n
		}},
	}
	for _, field := range fields {
		for _, value := range []json.Number{"NaN", "+Inf", "+1", "01", "heavy", "null", "1 2"} {
			t.Run(field.path+"/"+string(value), func(t *testing.T) {
				e := validEntry()
				e.Compounds[0].Bioactivities[0].Assays = []Assay{{
					Target: "test", Measurement: &AssayMeasurement{Concentration: "1"},
				}}
				field.set(e, value)
				issues := e.Validate()
				if len(issues) != 1 || issues[0].Path != field.path ||
					issues[0].Code != CodeInvalidValue || issues[0].Severity != SeverityError {
					t.Fatalf("expected one invalid-value error at %s, got %v", field.path, issues)
				}
			})
		}
	}
}

func TestValidJSONNumbersKeepTheirSpelling(t *testing.T) {
	for _, value := range []json.Number{"", "500.0", "1e-7", "1E+2"} {
		t.Run(string(value), func(t *testing.T) {
			e := validEntry()
			e.Compounds[0].Mass = value
			e.Compounds[0].Bioactivities[0].Assays = []Assay{{
				Concentration: "1", Target: "test",
				Measurement: &AssayMeasurement{Concentration: value, Error: value},
			}}
			if issues := e.Validate(); len(issues) != 0 {
				t.Fatalf("unexpected issues: %v", issues)
			}
			encoded, err := Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Parse(encoded)
			if err != nil {
				t.Fatal(err)
			}
			measurement := again.Compounds[0].Bioactivities[0].Assays[0].Measurement
			if again.Compounds[0].Mass != value || measurement.Concentration != value || measurement.Error != value {
				t.Fatalf("number spelling %q changed after encoding and parsing", value)
			}
		})
	}
}

type wantIssue struct {
	path     string
	code     IssueCode
	severity Severity
}

func TestValidationRules(t *testing.T) {
	tests := []struct {
		name    string
		quality Quality // validation quality, defaults to high
		mutate  func(e *Entry)
		want    []wantIssue
	}{
		{
			name:   "invalid accession",
			mutate: func(e *Entry) { e.Accession = "BGC123" },
			want:   []wantIssue{{"accession", CodeInvalidValue, SeverityError}},
		},
		{
			name:   "retired without reason",
			mutate: func(e *Entry) { e.Status = StatusRetired },
			want:   []wantIssue{{"retirement_reasons", CodeRequired, SeverityError}},
		},
		{
			name:   "no compounds",
			mutate: func(e *Entry) { e.Compounds = nil },
			want:   []wantIssue{{"compounds", CodeRequired, SeverityError}},
		},
		{
			name: "duplicate locus",
			mutate: func(e *Entry) {
				e.Loci = append(e.Loci, e.Loci[0])
			},
			want: []wantIssue{{"loci[1]", CodeDuplicate, SeverityError}},
		},
		{
			name:   "invalid citation",
			mutate: func(e *Entry) { e.Loci[0].Evidence[0].References = []Citation{"pubmed:abc", "isbn:123"} },
			want: []wantIssue{
				{"loci[0].evidence[0].references[0]", CodeInvalidValue, SeverityError},
				{"loci[0].evidence[0].references[1]", CodeInvalidValue, SeverityError},
			},
		},
		{
			name:   "missing citations",
			mutate: func(e *Entry) { e.Loci[0].Evidence[0].References = nil },
			want:   []wantIssue{{"loci[0].evidence[0].references", CodeMissingEvidence, SeverityError}},
		},
		{
			name:    "missing citations are fine for questionable entries",
			quality: QualityQuestionable,
			mutate:  func(e *Entry) { e.Loci[0].Evidence[0].References = nil },
		},
		{
			name: "sequence-based predictions need no citations",
			mutate: func(e *Entry) {
				e.Biosynthesis.Modules[0].(*NRPSType1Module).ADomain.Evidence[0] = Evidence{Method: "Sequence-based prediction"}
			},
		},
		{
			name:   "start after end",
			mutate: func(e *Entry) { e.Loci[0].Location = Location{From: 200, To: 100} },
			want:   []wantIssue{{"loci[0].location", CodeInconsistent, SeverityError}},
		},
		{
			name:    "start after end with negative coordinates",
			quality: QualityQuestionable,
			mutate:  func(e *Entry) { e.Loci[0].Location = Location{From: 10, To: -1} },
			want:    []wantIssue{{"loci[0].location", CodeInconsistent, SeverityError}},
		},
		{
			name: "ketide length zero",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, &PKSClass{Subclass: "Type I", KetideLength: Int(0)})
			},
		},
		{
			name: "negative ketide length",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, &PKSClass{Subclass: "Type I", KetideLength: Int(-1)})
			},
			want: []wantIssue{{"biosynthesis.classes[1].ketide_length", CodeInvalidValue, SeverityError}},
		},
		{
			name: "duplicates with different number spelling",
			mutate: func(e *Entry) {
				e.Compounds[0].Mass = "500"
				e.Compounds = append(e.Compounds, e.Compounds[0])
				e.Compounds[1].Mass = "500.0"
			},
			want: []wantIssue{{"compounds[1]", CodeDuplicate, SeverityError}},
		},
		{
			name: "huge exponents",
			mutate: func(e *Entry) {
				e.Compounds[0].Mass = "1e999999999"
				e.Compounds = append(e.Compounds, e.Compounds[0])
				e.Compounds[1].Mass = "1E999999999"
			},
			want: []wantIssue{
				{"compounds[0].mass", CodeInvalidValue, SeverityError},
				{"compounds[1].mass", CodeInvalidValue, SeverityError},
			},
		},
		{
			name: "large numbers are compared exactly",
			mutate: func(e *Entry) {
				e.Compounds[0].Mass = "9007199254740992"
				e.Compounds = append(e.Compounds, e.Compounds[0])
				e.Compounds[1].Mass = "9007199254740993"
			},
		},
		{
			name:   "empty optional list",
			mutate: func(e *Entry) { e.Genes = &Genes{ToAdd: []GeneAddition{}} },
			want:   []wantIssue{{"genes.to_add", CodeEmptyValue, SeverityWarning}},
		},
		{
			name:   "negative coordinates",
			mutate: func(e *Entry) { e.Loci[0].Location = Location{From: -1, To: -1} },
			want:   []wantIssue{{"loci[0].location", CodeInvalidValue, SeverityError}},
		},
		{
			name:    "negative coordinates are placeholders in questionable entries",
			quality: QualityQuestionable,
			mutate:  func(e *Entry) { e.Loci[0].Location = Location{From: -1, To: -1} },
		},
		{
			name:   "invalid evidence method",
			mutate: func(e *Entry) { e.Loci[0].Evidence[0].Method = "Guesswork" },
			want:   []wantIssue{{"loci[0].evidence[0].method", CodeInvalidValue, SeverityError}},
		},
		{
			name:   "high quality compounds need a structure",
			mutate: func(e *Entry) { e.Compounds[0].Structure = "" },
			want:   []wantIssue{{"compounds[0].structure", CodeRequired, SeverityError}},
		},
		{
			name:    "lower quality compounds do not need a structure",
			quality: QualityMedium,
			mutate:  func(e *Entry) { e.Compounds[0].Structure = "" },
		},
		{
			name:   "compound evidence Other needs details",
			mutate: func(e *Entry) { e.Compounds[0].Evidence[0].Method = "Other" },
			want:   []wantIssue{{"compounds[0].evidence[0].details", CodeRequired, SeverityError}},
		},
		{
			name: "compound evidence Other with details",
			mutate: func(e *Entry) {
				e.Compounds[0].Evidence[0] = CompoundEvidence{Method: "Other", Details: "UV", References: []Citation{"pubmed:1"}}
			},
		},
		{
			name: "invalid compound class and database ID",
			mutate: func(e *Entry) {
				e.Compounds[0].Classes = []string{"Alkaloid"}
				e.Compounds[0].DatabaseIDs = []DatabaseID{"pubchem:CID1"}
			},
			want: []wantIssue{
				{"compounds[0].classes[0]", CodeInvalidValue, SeverityError},
				{"compounds[0].databaseIds[0]", CodeInvalidValue, SeverityError},
			},
		},
		{
			name:   "unusual formula is a warning",
			mutate: func(e *Entry) { e.Compounds[0].Formula = "C2H6 . HCl" },
			want:   []wantIssue{{"compounds[0].formula", CodeInvalidValue, SeverityWarning}},
		},
		{
			name:   "legacy bioactivity name",
			mutate: func(e *Entry) { e.Compounds[0].Bioactivities[0].Name.Legacy = true },
			want:   []wantIssue{{"compounds[0].bioactivities[0].name", CodeLegacyFormat, SeverityWarning}},
		},
		{
			name:   "missing observed flag",
			mutate: func(e *Entry) { e.Compounds[0].Bioactivities[0].Observed = nil },
			want:   []wantIssue{{"compounds[0].bioactivities[0].observed", CodeRequired, SeverityError}},
		},
		{
			name:   "NRPS subclass",
			mutate: func(e *Entry) { e.Biosynthesis.Classes[0].(*NRPSClass).Subclass = "Type 1" },
			want:   []wantIssue{{"biosynthesis.classes[0].subclass", CodeInvalidValue, SeverityError}},
		},
		{
			name:    "NRPS subclass is not checked for questionable entries",
			quality: QualityQuestionable,
			mutate:  func(e *Entry) { e.Biosynthesis.Classes[0].(*NRPSClass).Subclass = "" },
		},
		{
			name: "unknown class",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes[0] = &UnknownClass{Name: "lipid", Raw: json.RawMessage(`{"class":"lipid"}`)}
			},
			want: []wantIssue{{"biosynthesis.classes[0].class", CodeUnknownType, SeverityError}},
		},
		{
			name:   "null class",
			mutate: func(e *Entry) { e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, nil) },
			want:   []wantIssue{{"biosynthesis.classes[1]", CodeRequired, SeverityError}},
		},
		{
			name: "RiPP details",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, &RibosomalClass{
					Subclass: "RiPP", RiPPType: "other",
					Precursors: []RiPPPrecursor{{Gene: "ripA", CoreSequence: CoreSequence{Sequences: []string{"ITS"}}}},
				})
			},
			want: []wantIssue{{"biosynthesis.classes[1].details", CodeRequired, SeverityError}},
		},
		{
			name: "RiPPs need precursors at medium quality",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, &RibosomalClass{Subclass: "unmodified"})
			},
			want: []wantIssue{{"biosynthesis.classes[1].precursors", CodeRequired, SeverityError}},
		},
		{
			name: "crosslinks",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, &RibosomalClass{
					Subclass: "unmodified",
					Precursors: []RiPPPrecursor{{
						Gene:         "ripA",
						CoreSequence: CoreSequence{Sequences: []string{"A", "B"}},
						Crosslinks:   []Crosslink{{From: 5, To: 3, Type: "Thioether"}},
					}},
				})
			},
			want: []wantIssue{
				{"biosynthesis.classes[1].precursors[0].core_sequence", CodeLegacyFormat, SeverityWarning},
				{"biosynthesis.classes[1].precursors[0].crosslinks[0]", CodeInconsistent, SeverityError},
				{"biosynthesis.classes[1].precursors[0].crosslinks[0].type", CodeInvalidValue, SeverityError},
			},
		},
		{
			name: "legacy terpene synthases",
			mutate: func(e *Entry) {
				e.Biosynthesis.Classes = append(e.Biosynthesis.Classes, &TerpeneClass{Subclass: "Diterpene", Synthases: []GeneID{"terA"}})
			},
			want: []wantIssue{{"biosynthesis.classes[1].synthases", CodeLegacyFormat, SeverityWarning}},
		},
		{
			name:   "module needs active",
			mutate: func(e *Entry) { e.Biosynthesis.Modules[0].Base().Active = nil },
			want:   []wantIssue{{"biosynthesis.modules[0].active", CodeRequired, SeverityError}},
		},
		{
			name: "core domains are not modification domains",
			mutate: func(e *Entry) {
				m := e.Biosynthesis.Modules[0].(*NRPSType1Module)
				m.ModificationDomains = Domains{
					&CondensationDomain{DomainBase: DomainBase{Gene: "nrpA", Location: Location{From: 1, To: 5}}},
					&EpimeraseDomain{DomainBase: DomainBase{Gene: "nrpA", Location: Location{From: 500, To: 800}}},
				}
			},
			want: []wantIssue{{"biosynthesis.modules[0].modification_domains[0].type", CodeNotAllowed, SeverityError}},
		},
		{
			name: "substrates need evidence",
			mutate: func(e *Entry) {
				e.Biosynthesis.Modules[0].(*NRPSType1Module).ADomain.Evidence = nil
			},
			want: []wantIssue{{"biosynthesis.modules[0].a_domain.evidence", CodeMissingEvidence, SeverityError}},
		},
		{
			name: "inactive domains have no substrates",
			mutate: func(e *Entry) {
				e.Biosynthesis.Modules[0].(*NRPSType1Module).ADomain.Inactive = Bool(true)
			},
			want: []wantIssue{{"biosynthesis.modules[0].a_domain.substrates", CodeInconsistent, SeverityError}},
		},
		{
			name: "proteinogenic substrates",
			mutate: func(e *Entry) {
				e.Biosynthesis.Modules[0].(*NRPSType1Module).ADomain.Substrates[0].Name = "ornithine"
			},
			want: []wantIssue{{"biosynthesis.modules[0].a_domain.substrates[0].name", CodeInvalidValue, SeverityError}},
		},
		{
			name: "gene IDs",
			mutate: func(e *Entry) {
				e.Biosynthesis.Modules[0].Base().Genes = []GeneID{"nrp A", "nrp(A)"}
			},
			want: []wantIssue{
				{"biosynthesis.modules[0].genes[0]", CodeInvalidValue, SeverityError},
				{"biosynthesis.modules[0].genes[1]", CodeInvalidValue, SeverityError},
			},
		},
		{
			name:    "multi-locus gene IDs are fine for questionable entries",
			quality: QualityQuestionable,
			mutate: func(e *Entry) {
				e.Biosynthesis.Modules[0].Base().Genes = []GeneID{"nrpA(AB000001.1:1-100)"}
			},
		},
		{
			name: "gene additions",
			mutate: func(e *Entry) {
				e.Genes = &Genes{ToAdd: []GeneAddition{{
					ID:          "orf1",
					Location:    GeneLocation{Exons: []Location{{From: 1, To: 300}}, Strand: 0},
					Translation: new(string),
				}}}
			},
			want: []wantIssue{
				{"genes.to_add[0].location.strand", CodeInvalidValue, SeverityError},
				{"genes.to_add[0].translation", CodeRequired, SeverityError},
			},
		},
		{
			name: "gene function Other",
			mutate: func(e *Entry) {
				e.Genes = &Genes{Annotations: []GeneAnnotation{{
					ID:        "nrpA",
					Functions: []GeneFunction{{Function: FunctionCategory{Name: "Other"}, Evidence: []Evidence{{Method: "Knock-out", References: []Citation{"pubmed:1"}}}}},
					Tailoring: []TailoringFunction{{Function: "Oxidation", References: []Citation{"pubmed:1"}, DBReference: "MITE0000001"}},
				}}}
			},
			want: []wantIssue{
				{"genes.annotations[0].functions[0].function.details", CodeRequired, SeverityError},
				{"genes.annotations[0].tailoring[0].db_reference", CodeInvalidValue, SeverityError},
			},
		},
		{
			name:   "legacy references need questionable quality",
			mutate: func(e *Entry) { e.LegacyReferences = []Citation{"pubmed:1"} },
			want:   []wantIssue{{"legacy_references", CodeNotAllowed, SeverityError}},
		},
		{
			name: "unknown keys",
			mutate: func(e *Entry) {
				e.Extra = Extras{Unknown: []UnknownField{{Key: "colour", Value: json.RawMessage(`"red"`)}, {Key: "leftover", Value: json.RawMessage(`[]`)}}}
				e.Taxonomy.Extra = Extras{Unknown: []UnknownField{{Key: "rank", Value: json.RawMessage(`null`)}}}
			},
			want: []wantIssue{
				{"colour", CodeUnknownField, SeverityError},
				{"leftover", CodeUnknownField, SeverityWarning},
				{"taxonomy.rank", CodeUnknownField, SeverityWarning},
			},
		},
		{
			name: "preserved empty values",
			mutate: func(e *Entry) {
				e.Compounds[0].Extra = Extras{preserved: []preservedField{{field: "formula", key: "formula", value: json.RawMessage(`""`)}}}
			},
			want: []wantIssue{{"compounds[0].formula", CodeEmptyValue, SeverityWarning}},
		},
		{
			name: "changelog",
			mutate: func(e *Entry) {
				e.Changelog.Releases[0].Version = "v4"
				e.Changelog.Releases[0].Entries[0].Reviewers = []SubmitterID{"short"}
			},
			want: []wantIssue{
				{"changelog.releases[0].version", CodeInvalidValue, SeverityError},
				{"changelog.releases[0].entries[0].reviewers[0]", CodeInvalidValue, SeverityError},
			},
		},
		{
			name:   "the next release has no date",
			mutate: func(e *Entry) { e.Changelog.Releases[0].Version = ReleaseNext; e.Changelog.Releases[0].Date = "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEntry()
			tt.mutate(e)
			quality := tt.quality
			if quality == "" {
				quality = QualityHigh
			}
			var got []wantIssue
			for _, issue := range e.ValidateAt(quality) {
				got = append(got, wantIssue{issue.Path, issue.Code, issue.Severity})
			}
			sortIssues(got)
			sortIssues(tt.want)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got issues\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

func sortIssues(issues []wantIssue) {
	slices.SortFunc(issues, func(a, b wantIssue) int { return strings.Compare(a.path+string(a.code), b.path+string(b.code)) })
}

func TestValidateParsedEntries(t *testing.T) {
	tests := []struct {
		name  string
		input func(e map[string]any)
		want  []wantIssue
	}{
		{
			name: "missing required keys",
			input: func(e map[string]any) {
				delete(e, "taxonomy")
				e["loci"].([]any)[0].(map[string]any)["location"] = map[string]any{}
			},
			want: []wantIssue{
				{"loci[0].location.from", CodeRequired, SeverityError},
				{"loci[0].location.to", CodeRequired, SeverityError},
				{"taxonomy", CodeRequired, SeverityError},
				{"taxonomy.name", CodeRequired, SeverityError},
				{"taxonomy.ncbiTaxId", CodeInvalidValue, SeverityError},
			},
		},
		{
			name: "null for required values",
			input: func(e map[string]any) {
				e["loci"].([]any)[0].(map[string]any)["location"] = nil
				e["changelog"].(map[string]any)["releases"].([]any)[0].(map[string]any)["entries"].([]any)[0].(map[string]any)["comment"] = nil
				e["taxonomy"].(map[string]any)["ncbiTaxId"] = nil
			},
			want: []wantIssue{
				{"changelog.releases[0].entries[0].comment", CodeRequired, SeverityError},
				{"loci[0].location", CodeRequired, SeverityError},
				{"taxonomy.ncbiTaxId", CodeInvalidValue, SeverityError},
			},
		},
		{
			name: "release type without references",
			input: func(e map[string]any) {
				e["biosynthesis"].(map[string]any)["classes"] = []any{map[string]any{
					"class": "NRPS", "subclass": "Type I",
					"release_types": []any{map[string]any{"name": "Hydrolysis"}},
				}}
			},
		},
		{
			name: "empty values and odd keys",
			input: func(e map[string]any) {
				e["comment"] = ""
				e["odd.key"] = "x"
			},
			want: []wantIssue{
				{"comment", CodeEmptyValue, SeverityWarning},
				{`["odd.key"]`, CodeUnknownField, SeverityError},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(validEntry())
			var generic map[string]any
			if err := json.Unmarshal(data, &generic); err != nil {
				t.Fatal(err)
			}
			tt.input(generic)
			data, _ = json.Marshal(generic)
			e, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			var got []wantIssue
			for _, issue := range e.Validate() {
				got = append(got, wantIssue{issue.Path, issue.Code, issue.Severity})
			}
			sortIssues(got)
			sortIssues(tt.want)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got issues\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

func TestEditingClearsEmptyValueWarnings(t *testing.T) {
	data, _ := json.Marshal(validEntry())
	data = []byte(strings.Replace(string(data), `"accession":"BGC0000001"`, `"accession":"BGC0000001","comment":""`, 1))
	e, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Validate()) != 1 {
		t.Fatalf("expected an empty-value warning, got %v", e.Validate())
	}
	e.Comment = "now set"
	if issues := e.Validate(); len(issues) != 0 {
		t.Errorf("expected no issues after setting the field, got %v", issues)
	}
}

func TestValidateUsesEntryQuality(t *testing.T) {
	e := validEntry()
	e.Loci[0].Evidence[0].References = nil
	if !e.Validate().HasErrors() {
		t.Error("expected errors at quality high")
	}
	e.Quality = QualityQuestionable
	if issues := e.Validate(); len(issues) != 0 {
		t.Errorf("expected no issues at quality questionable, got %v", issues)
	}
	if !e.ValidateAt(QualityLow).HasErrors() {
		t.Error("expected errors when validating at quality low")
	}
}

func TestIssues(t *testing.T) {
	issues := Issues{
		{Path: "a.b[2].c", Code: CodeRequired, Severity: SeverityError, Message: "c is required"},
		{Path: "x", Code: CodeLegacyFormat, Severity: SeverityWarning, Message: "old"},
	}
	if len(issues.Errors()) != 1 || len(issues.Warnings()) != 1 {
		t.Errorf("Errors/Warnings split wrong")
	}
	err := issues.Err()
	var verr *ValidationError
	if !errors.As(err, &verr) || len(verr.Issues) != 2 {
		t.Fatalf("Err() = %v", err)
	}
	if want := "validation failed: a.b[2].c: error: c is required [required]"; err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
	if issues.Warnings().Err() != nil {
		t.Error("warnings alone must not be an error")
	}
	if got := issues[0].Pointer(); got != "/a/b/2/c" {
		t.Errorf("Pointer() = %q", got)
	}
	pointers := map[string]string{
		"x":                   "/x",
		`a["b.c"][3]["x/y~"]`: "/a/b.c/3/x~1y~0",
		`[""]`:                "/",
		"":                    "",
	}
	for path, want := range pointers {
		if got := (Issue{Path: path}).Pointer(); got != want {
			t.Errorf("Pointer() of %q = %q, want %q", path, got, want)
		}
	}
	encoded, _ := json.Marshal(issues[1])
	if want := `{"path":"x","code":"legacy-format","severity":"warning","message":"old"}`; string(encoded) != want {
		t.Errorf("got %s", encoded)
	}
	var decoded Issue
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != issues[1] {
		t.Errorf("decoding gave %+v, %v", decoded, err)
	}
	if err := json.Unmarshal([]byte(`{"severity":"fatal"}`), &decoded); err == nil {
		t.Error("expected an error for an unknown severity")
	}
}

func TestReferences(t *testing.T) {
	e := validEntry()
	e.LegacyReferences = []Citation{"doi:10.1000/xyz", "pubmed:12345"}
	got := e.References()
	want := []Citation{"doi:10.1000/xyz", "pubmed:12345"}
	if !slices.Equal(got, want) {
		t.Errorf("References() = %v, want %v", got, want)
	}
}

func TestQuality(t *testing.T) {
	if !QualityHigh.AtLeast(QualityMedium) || QualityLow.AtLeast(QualityMedium) || !QualityLow.AtLeast(QualityLow) {
		t.Error("AtLeast ordering is wrong")
	}
	if Quality("excellent").Valid() || Quality("excellent").AtLeast(QualityQuestionable) {
		t.Error("unknown quality levels must rank lowest")
	}
}
