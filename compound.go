package mibig

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// Compound is a compound produced by the gene cluster.
type Compound struct {
	Name     string             `json:"name"`
	Evidence []CompoundEvidence `json:"evidence"`
	// Classes are chemical compound classes, e.g. "Lipopeptide".
	Classes       []string      `json:"classes,omitzero"`
	Bioactivities []Bioactivity `json:"bioactivities,omitzero"`
	Structure     SMILES        `json:"structure,omitzero"`
	// PartialStructure marks structures that are not fully determined.
	PartialStructure *bool        `json:"partial_structure,omitzero"`
	Synonyms         []string     `json:"synonyms,omitzero"`
	DatabaseIDs      []DatabaseID `json:"databaseIds,omitzero"`
	Moieties         []string     `json:"moieties,omitzero"`
	// Targets is a placeholder of the schema, without a defined format.
	Targets []json.RawMessage `json:"targets,omitzero"`
	Cyclic  *bool             `json:"cyclic,omitzero"`
	// Mass is the monoisotopic mass in Dalton. It is a json.Number so that it
	// is written back exactly as read, e.g. "500" or "500.0"; use
	// Mass.Float64() to get its value.
	Mass json.Number `json:"mass,omitzero"`
	// Formula is the molecular formula, e.g. "C10H12N2O".
	Formula string `json:"formula,omitzero"`
	Extra   Extras `json:"-"`
}

var (
	// compoundNamePattern combines the schema pattern with the extra
	// characters the Python implementation allows.
	compoundNamePattern = regexp.MustCompile(`^[a-zA-Zα-ωΑ-Ω0-9\[\]{}'()/&,.:; +-]+$`)
	formulaPattern      = regexp.MustCompile(`^([A-Z][a-z]?\d*)+([+-]\d*)?$`)
)

func (c *Compound) validate(v *validator, path string) {
	if v.required(join(path, "name"), c.Name, "compound name") && !compoundNamePattern.MatchString(c.Name) {
		v.errorf(join(path, "name"), CodeInvalidValue, "compound name %q contains invalid characters", c.Name)
	}

	evidencePath := join(path, "evidence")
	if len(c.Evidence) == 0 && v.strict() {
		v.errorf(evidencePath, CodeMissingEvidence, "compound evidence is required at quality %q", v.quality)
	}
	for i := range c.Evidence {
		c.Evidence[i].validate(v, index(evidencePath, i))
	}
	unique(v, evidencePath, c.Evidence)

	for i, class := range c.Classes {
		v.oneOf(index(join(path, "classes"), i), class, compoundClasses, "compound class")
	}
	unique(v, join(path, "classes"), c.Classes)

	for i := range c.Bioactivities {
		c.Bioactivities[i].validate(v, index(join(path, "bioactivities"), i))
	}

	if c.Structure != "" {
		c.Structure.validate(v, join(path, "structure"))
	} else if v.atLeast(QualityHigh) {
		v.errorf(join(path, "structure"), CodeRequired, "a structure is required at quality %q", v.quality)
	}

	for i, synonym := range c.Synonyms {
		if !compoundNamePattern.MatchString(synonym) {
			v.errorf(index(join(path, "synonyms"), i), CodeInvalidValue, "synonym %q contains invalid characters", synonym)
		}
	}
	unique(v, join(path, "synonyms"), c.Synonyms)

	for i, id := range c.DatabaseIDs {
		id.validate(v, index(join(path, "databaseIds"), i))
	}
	unique(v, join(path, "databaseIds"), c.DatabaseIDs)

	for i, moiety := range c.Moieties {
		if !compoundNamePattern.MatchString(moiety) {
			v.errorf(index(join(path, "moieties"), i), CodeInvalidValue, "moiety %q contains invalid characters", moiety)
		}
	}

	if c.Mass != "" && v.number(join(path, "mass"), c.Mass) {
		if mass, err := c.Mass.Float64(); err != nil || mass <= 0 {
			v.errorf(join(path, "mass"), CodeInvalidValue, "mass must be a positive number, got %s", c.Mass)
		}
	}
	if c.Formula != "" && !formulaPattern.MatchString(c.Formula) {
		v.warnf(join(path, "formula"), CodeInvalidValue, "unusual molecular formula %q", c.Formula)
	}
}

// CompoundEvidence describes how a compound's structure was elucidated.
type CompoundEvidence struct {
	Method string `json:"method"`
	// Details describes the method; it is required for method "Other". Not
	// part of the schema.
	Details    string     `json:"details,omitzero"`
	References []Citation `json:"references"`
	Extra      Extras     `json:"-"`
}

func (e *CompoundEvidence) validate(v *validator, path string) {
	v.oneOf(join(path, "method"), e.Method, compoundEvidenceMethods, "compound evidence method")
	if e.Method == "Other" {
		v.required(join(path, "details"), e.Details, `details for method "Other"`)
	}
	v.citations(join(path, "references"), e.References, true)
}

// DatabaseID is a reference to a chemical database, written as
// "<database>:<identifier>", e.g. "pubchem:5282253" or "npatlas:NPA008347".
type DatabaseID string

// Database returns the database part of the ID, e.g. "pubchem".
func (d DatabaseID) Database() string {
	database, _, _ := strings.Cut(string(d), ":")
	return database
}

// ID returns the identifier part of the ID, e.g. "5282253".
func (d DatabaseID) ID() string {
	_, id, _ := strings.Cut(string(d), ":")
	return id
}

var databaseIDPatterns = map[string]*regexp.Regexp{
	"pubchem":    regexp.MustCompile(`^\d+$`),
	"chebi":      regexp.MustCompile(`^\d+$`),
	"chembl":     regexp.MustCompile(`^CHEMBL\d+$`),
	"chemspider": regexp.MustCompile(`^\d+$`),
	"npatlas":    regexp.MustCompile(`^NPA\d+$`),
	"lotus":      regexp.MustCompile(`^Q\d+$`),
	"gnps":       regexp.MustCompile(`^MSV\d+$`),
	"cyanometdb": regexp.MustCompile(`^CyanoMetDB_\d{4}$`),
}

func (d DatabaseID) validate(v *validator, path string) {
	pattern, known := databaseIDPatterns[d.Database()]
	switch {
	case !known:
		v.errorf(path, CodeInvalidValue, "unknown compound database in %q", d)
	case !pattern.MatchString(d.ID()):
		v.errorf(path, CodeInvalidValue, "invalid %s ID %q", d.Database(), d)
	}
}

// Bioactivity is a biological activity of a compound.
type Bioactivity struct {
	Name BioactivityName `json:"name"`
	// Observed is false for activities that were tested for, but not seen.
	Observed   *bool      `json:"observed"`
	References []Citation `json:"references"`
	Assays     []Assay    `json:"assays,omitzero" mibig:"nonempty"`
	Details    string     `json:"details,omitzero"`
	Extra      Extras     `json:"-"`
}

func (b *Bioactivity) validate(v *validator, path string) {
	b.Name.validate(v, join(path, "name"))
	v.requiredBool(join(path, "observed"), b.Observed, "observed")
	v.citations(join(path, "references"), b.References, true)
	for i := range b.Assays {
		b.Assays[i].validate(v, index(join(path, "assays"), i))
	}
	unique(v, join(path, "assays"), b.Assays)
}

// BioactivityName is the name of a bioactivity, e.g. "antibacterial".
//
// Current entries store it as a string. Older entries use the legacy form
// {"activity": "...", "comment": "..."}; Legacy records that, so the name is
// written back the same way. The validator warns about the legacy form.
type BioactivityName struct {
	Activity string `json:"activity"`
	// Comment is only available in the legacy form.
	Comment string `json:"comment,omitzero"`
	Legacy  bool   `json:"-"`
	// Extra holds unknown keys of the legacy form.
	Extra Extras `json:"-"`
}

func (n BioactivityName) String() string {
	return n.Activity
}

// IsZero reports whether the name is empty.
func (n BioactivityName) IsZero() bool {
	return n.Activity == "" && n.Comment == "" && !n.Legacy && n.Extra.IsZero()
}

// MarshalJSON writes the name as a string, or in the legacy object form if
// Legacy is set or there is a comment.
func (n BioactivityName) MarshalJSON() ([]byte, error) {
	if n.Legacy || n.Comment != "" || !n.Extra.IsZero() {
		return encodeObject(n)
	}
	return marshalNoEscape(n.Activity)
}

// UnmarshalJSON reads both the string and the legacy object form.
func (n *BioactivityName) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if err := decodeObject(trimmed, n); err != nil {
			return err
		}
		n.Legacy = true
		return nil
	}
	*n = BioactivityName{}
	return json.Unmarshal(data, &n.Activity)
}

func (n *BioactivityName) validate(v *validator, path string) {
	v.required(path, n.Activity, "bioactivity name")
	if n.Legacy || n.Comment != "" {
		v.warnf(path, CodeLegacyFormat, "bioactivity name uses the legacy object form, use a string and move the comment to details")
	}
}

// Assay is a quantitative bioactivity measurement.
//
// The data stores the concentration as a free text string; the schema
// defines a structured Measurement instead. Both are supported.
type Assay struct {
	Concentration string `json:"concentration,omitzero"`
	Target        string `json:"target"`
	// The following fields are defined by the schema, but not used yet.
	Measurement *AssayMeasurement `json:"measurement,omitzero"`
	// Type is a placeholder of the schema, without a defined format.
	Type       json.RawMessage `json:"type,omitzero"`
	Details    string          `json:"details,omitzero"`
	TestSystem *TestSystem     `json:"test_system,omitzero"`
	References []Citation      `json:"references,omitzero" mibig:"nonempty"`
	Extra      Extras          `json:"-"`
}

func (a *Assay) validate(v *validator, path string) {
	if a.Concentration == "" && (a.Measurement == nil || a.Measurement.Concentration == "") {
		v.errorf(join(path, "concentration"), CodeRequired, "assay concentration is required")
	}
	if a.Measurement != nil {
		measurementPath := join(path, "measurement")
		v.number(join(measurementPath, "concentration"), a.Measurement.Concentration)
		v.number(join(measurementPath, "error"), a.Measurement.Error)
	}
	v.required(join(path, "target"), a.Target, "assay target")
	v.citations(join(path, "references"), a.References, false)
}

// AssayMeasurement is a structured assay result.
type AssayMeasurement struct {
	Concentration json.Number `json:"concentration,omitzero"`
	Unit          string      `json:"unit,omitzero"`
	Error         json.Number `json:"error,omitzero"`
	Replicates    *int        `json:"replicates,omitzero"`
	Extra         Extras      `json:"-"`
}

// TestSystem describes the system an assay was run in.
type TestSystem struct {
	CellLine string `json:"cell_line,omitzero"`
	// Organism is an NCBI taxonomy ID.
	Organism *int64 `json:"organism,omitzero"`
	Strain   string `json:"strain,omitzero"`
	Extra    Extras `json:"-"`
}
