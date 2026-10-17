package mibig

import (
	"regexp"
	"strconv"
	"strings"
)

// Quality is the annotation quality level of an entry.
type Quality string

const (
	QualityQuestionable Quality = "questionable"
	QualityLow          Quality = "low"
	QualityMedium       Quality = "medium"
	QualityHigh         Quality = "high"
)

var qualityRanks = map[Quality]int{
	QualityQuestionable: 0,
	QualityLow:          1,
	QualityMedium:       2,
	QualityHigh:         3,
}

// Valid reports whether q is a known quality level.
func (q Quality) Valid() bool {
	_, ok := qualityRanks[q]
	return ok
}

// AtLeast reports whether q is at least as good as other. Unknown quality
// levels rank below questionable.
func (q Quality) AtLeast(other Quality) bool {
	return q.rank() >= other.rank()
}

func (q Quality) rank() int {
	if rank, ok := qualityRanks[q]; ok {
		return rank
	}
	return -1
}

var qualities = vocabulary{
	string(QualityQuestionable), string(QualityLow), string(QualityMedium), string(QualityHigh),
}

// Status is the publication status of an entry.
type Status string

const (
	StatusPending   Status = "pending"
	StatusEmbargoed Status = "embargoed"
	StatusActive    Status = "active"
	StatusRetired   Status = "retired"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusEmbargoed, StatusActive, StatusRetired:
		return true
	}
	return false
}

var statuses = vocabulary{
	string(StatusPending), string(StatusEmbargoed), string(StatusActive), string(StatusRetired),
}

// Completeness describes whether the locus covers the whole gene cluster.
type Completeness string

const (
	CompletenessUnknown  Completeness = "unknown"
	CompletenessPartial  Completeness = "partial"
	CompletenessComplete Completeness = "complete"
)

var completenesses = vocabulary{
	string(CompletenessUnknown), string(CompletenessPartial), string(CompletenessComplete),
}

// Valid reports whether c is a known completeness level.
func (c Completeness) Valid() bool {
	switch c {
	case CompletenessUnknown, CompletenessPartial, CompletenessComplete:
		return true
	}
	return false
}

// Entry is a MIBiG entry, describing one biosynthetic gene cluster.
type Entry struct {
	// Accession is the MIBiG accession, e.g. "BGC0000001".
	Accession    string       `json:"accession"`
	Version      int          `json:"version"`
	Changelog    Changelog    `json:"changelog"`
	Quality      Quality      `json:"quality"`
	Status       Status       `json:"status"`
	Completeness Completeness `json:"completeness"`
	Loci         []Locus      `json:"loci"`
	Biosynthesis Biosynthesis `json:"biosynthesis"`
	Compounds    []Compound   `json:"compounds"`
	Taxonomy     Taxonomy     `json:"taxonomy"`
	Genes        *Genes       `json:"genes,omitzero"`
	// RetirementReasons explain why a retired entry was retired.
	RetirementReasons []string `json:"retirement_reasons,omitzero"`
	// SeeAlso lists accessions of related entries.
	SeeAlso []string `json:"see_also,omitzero"`
	Comment string   `json:"comment,omitzero"`
	// LegacyReferences are citations carried over from older versions of
	// the entry that are not attached to a specific claim. Not part of the
	// schema; only allowed for entries of questionable quality.
	LegacyReferences []Citation `json:"legacy_references,omitzero"`
	Extra            Extras     `json:"-"`
}

// AccessionVersion returns the versioned accession, e.g. "BGC0000001.5".
func (e *Entry) AccessionVersion() string {
	return e.Accession + "." + strconv.Itoa(e.Version)
}

// Validate checks the entry against the schema and the semantic rules, at
// the entry's own quality level.
func (e *Entry) Validate() Issues {
	return e.ValidateAt(e.Quality)
}

// ValidateAt checks the entry as if it had quality level q. This can be used
// to check whether an entry qualifies for a higher quality level. Unknown
// quality levels are validated like QualityLow.
func (e *Entry) ValidateAt(q Quality) Issues {
	if !q.Valid() {
		q = QualityLow
	}
	v := &validator{quality: q}
	e.validate(v)
	validateExtras(v, e)
	return v.issues
}

var (
	accessionPattern = regexp.MustCompile(`^(BGC\d{7}|new\d{3})$`)
	seeAlsoPattern   = regexp.MustCompile(`^BGC\d{7}$`)
)

func (e *Entry) validate(v *validator) {
	if !accessionPattern.MatchString(e.Accession) {
		v.errorf("accession", CodeInvalidValue, "invalid accession %q, expected BGC followed by 7 digits", e.Accession)
	}
	if e.Version < 1 {
		v.errorf("version", CodeInvalidValue, "version must be at least 1, got %d", e.Version)
	}
	e.Changelog.validate(v, "changelog")

	v.recordVocabulary("quality", qualities)
	v.recordVocabulary("status", statuses)
	v.recordVocabulary("completeness", completenesses)
	if !e.Quality.Valid() {
		v.errorf("quality", CodeInvalidValue, "%q is not a valid quality level", e.Quality)
	}
	if !e.Status.Valid() {
		v.errorf("status", CodeInvalidValue, "%q is not a valid status", e.Status)
	}
	if !e.Completeness.Valid() {
		v.errorf("completeness", CodeInvalidValue, "%q is not a valid completeness", e.Completeness)
	}

	requiredList(v, "loci", e.Loci, "locus")
	for i := range e.Loci {
		e.Loci[i].validate(v, index("loci", i))
	}
	unique(v, "loci", e.Loci)

	e.Biosynthesis.validate(v, "biosynthesis")

	requiredList(v, "compounds", e.Compounds, "compound")
	for i := range e.Compounds {
		e.Compounds[i].validate(v, index("compounds", i))
	}
	unique(v, "compounds", e.Compounds)

	e.Taxonomy.validate(v, "taxonomy")

	if e.Genes != nil {
		e.Genes.validate(v, "genes")
	}

	if e.Status == StatusRetired && len(e.RetirementReasons) == 0 {
		v.errorf("retirement_reasons", CodeRequired, "retired entries need a retirement reason")
	}
	for i, reason := range e.RetirementReasons {
		v.required(index("retirement_reasons", i), reason, "retirement reason")
	}
	unique(v, "retirement_reasons", e.RetirementReasons)

	for i, accession := range e.SeeAlso {
		if !seeAlsoPattern.MatchString(accession) {
			v.errorf(index("see_also", i), CodeInvalidValue, "invalid accession %q", accession)
		}
	}
	unique(v, "see_also", e.SeeAlso)

	if len(e.LegacyReferences) > 0 && v.strict() {
		v.errorf("legacy_references", CodeNotAllowed, "legacy references are only allowed for entries of questionable quality")
	}
	v.citations("legacy_references", e.LegacyReferences, false)
}

// Taxonomy is the producing organism.
type Taxonomy struct {
	Name      string `json:"name"`
	NCBITaxID int64  `json:"ncbiTaxId"`
	Extra     Extras `json:"-"`
}

func (t *Taxonomy) validate(v *validator, path string) {
	v.required(join(path, "name"), t.Name, "organism name")
	if t.NCBITaxID < 1 {
		v.errorf(join(path, "ncbiTaxId"), CodeInvalidValue, "invalid NCBI taxonomy ID %d", t.NCBITaxID)
	}
}

// Locus is a region of a sequence record containing the gene cluster.
type Locus struct {
	// Accession is the accession of the sequence record, e.g. "AB012345.1".
	Accession string     `json:"accession"`
	Location  Location   `json:"location"`
	Evidence  []Evidence `json:"evidence"`
	// DraftGenome marks loci on draft genome assemblies. Not part of the
	// schema.
	DraftGenome *bool  `json:"draft_genome,omitzero"`
	Extra       Extras `json:"-"`
}

func (l *Locus) validate(v *validator, path string) {
	if v.required(join(path, "accession"), l.Accession, "locus accession") {
		if strings.Count(l.Accession, ".") > 1 && !strings.HasPrefix(l.Accession, "MIBIG.") {
			v.errorf(join(path, "accession"), CodeInvalidValue, "invalid locus accession %q", l.Accession)
		}
	}
	l.Location.validate(v, join(path, "location"))
	validateEvidence(v, join(path, "evidence"), l.Evidence, locusEvidenceMethods, true)
}
