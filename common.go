package mibig

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// Bool returns a pointer to b, for filling in optional boolean fields.
func Bool(b bool) *bool {
	return &b
}

// Int returns a pointer to i, for filling in optional integer fields.
func Int(i int) *int {
	return &i
}

// Citation is a reference to a publication or web resource, written as
// "<database>:<identifier>", e.g. "pubmed:12345678",
// "doi:10.1093/nar/gkac1049", "patent:US1234567" or "url:https://example.org".
type Citation string

// Database returns the database part of the citation, e.g. "pubmed".
func (c Citation) Database() string {
	database, _, _ := strings.Cut(string(c), ":")
	return database
}

// ID returns the identifier part of the citation, e.g. "12345678".
func (c Citation) ID() string {
	_, id, _ := strings.Cut(string(c), ":")
	return id
}

// URL returns a URL for the citation, or an empty string for unknown
// databases.
func (c Citation) URL() string {
	id := c.ID()
	switch c.Database() {
	case "pubmed":
		return "https://pubmed.ncbi.nlm.nih.gov/" + id + "/"
	case "doi":
		return "https://doi.org/" + id
	case "patent":
		return "https://patents.google.com/patent/" + id
	case "url":
		return id
	}
	return ""
}

var citationPatterns = map[string]*regexp.Regexp{
	"pubmed": regexp.MustCompile(`^\d+$`),
	// The schema does not allow <>, but some publishers (e.g. ANIE) use them.
	"doi":    regexp.MustCompile(`^10\.\d{4,9}/[-._;()<>/:a-zA-Z0-9]+$`),
	"patent": regexp.MustCompile(`^.+$`),
	"url":    regexp.MustCompile(`^https?://(www\.)?[-a-zA-Z0-9@:%._+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_+.~#?&/=]*)$`),
}

func (c Citation) validate(v *validator, path string) {
	database, id, found := strings.Cut(string(c), ":")
	pattern, known := citationPatterns[database]
	switch {
	case !found || !known:
		v.errorf(path, CodeInvalidValue, "invalid citation %q, expected one of pubmed:, doi:, patent: or url:", c)
	case !pattern.MatchString(id):
		v.errorf(path, CodeInvalidValue, "invalid %s citation %q", database, c)
	}
}

// GeneID identifies a gene, usually by its locus tag, gene name or protein ID.
type GeneID string

// geneIDInvalidChars are characters that are not allowed in gene IDs of
// entries with a quality better than questionable.
var geneIDInvalidChars = regexp.MustCompile("[!?,;:=+*&^%$#@ \t\n\r\\\\/\\[\\]{}()<>|~`'\"]")

func (g GeneID) validate(v *validator, path string) {
	switch {
	case g == "":
		v.errorf(path, CodeRequired, "gene ID must not be empty")
	case strings.ContainsAny(string(g), ", "):
		v.errorf(path, CodeInvalidValue, "gene ID %q must not contain commas or spaces", g)
	case v.strict() && geneIDInvalidChars.MatchString(string(g)):
		v.errorf(path, CodeInvalidValue, "gene ID %q contains invalid characters", g)
	}
}

// SMILES is a chemical structure in SMILES notation.
type SMILES string

var smilesPattern = regexp.MustCompile(`^[\[\]a-zA-Z0-9@()=/\\#+.%*-]+$`)

func (s SMILES) validate(v *validator, path string) {
	if !smilesPattern.MatchString(string(s)) {
		v.errorf(path, CodeInvalidValue, "invalid SMILES %q", s)
	}
}

// Location is a 1-based, inclusive range on a sequence. Loci use nucleotide
// coordinates, domains and cleavage sites use amino acid coordinates.
//
// The data uses 0/0 for "not specified" in places, and -1/-1 as a placeholder
// in entries of questionable quality.
type Location struct {
	From  int    `json:"from"`
	To    int    `json:"to"`
	Extra Extras `json:"-"`
}

func (l *Location) validate(v *validator, path string) {
	if (l.From < 0 || l.To < 0) && v.strict() {
		v.errorf(path, CodeInvalidValue, "coordinates must not be negative, got %d-%d", l.From, l.To)
	}
	if l.From > l.To {
		v.errorf(path, CodeInconsistent, "start %d is after end %d", l.From, l.To)
	}
}

// Date is a calendar date in "YYYY-MM-DD" format. An empty Date is written as
// null.
type Date string

// Time parses the date.
func (d Date) Time() (time.Time, error) {
	return time.Parse(time.DateOnly, string(d))
}

// MarshalJSON writes an empty date as null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d == "" {
		return []byte("null"), nil
	}
	return json.Marshal(string(d))
}

// UnmarshalJSON reads null as an empty date.
func (d *Date) UnmarshalJSON(data []byte) error {
	if isNull(data) {
		*d = ""
		return nil
	}
	return json.Unmarshal(data, (*string)(d))
}

func (d Date) validate(v *validator, path string) {
	if d == "" {
		v.errorf(path, CodeRequired, "date is required")
		return
	}
	if _, err := d.Time(); err != nil {
		v.errorf(path, CodeInvalidValue, "invalid date %q, expected YYYY-MM-DD", d)
	}
}

// Evidence supports a claim with an experimental or computational method and
// the publications describing it. The valid methods depend on what the
// evidence is for (loci, substrates, gene functions, operons, ...).
type Evidence struct {
	// Method is the method used. Some schema files call this "name".
	Method     string     `json:"method" mibig:"alias=name"`
	References []Citation `json:"references,omitzero"`
	Extra      Extras     `json:"-"`
}

// methodSequenceBased is the one evidence method that does not need citations.
const methodSequenceBased = "Sequence-based prediction"

func (e *Evidence) validate(v *validator, path string, methods vocabulary) {
	v.oneOf(join(path, "method"), e.Method, methods, "evidence method")
	v.citations(join(path, "references"), e.References, e.Method != methodSequenceBased)
}

// validateEvidence validates a list of evidence. If required is set, the list
// must not be empty unless the entry is of questionable quality.
func validateEvidence(v *validator, path string, evidence []Evidence, methods vocabulary, required bool) {
	if required && len(evidence) == 0 && v.strict() {
		v.errorf(path, CodeMissingEvidence, "evidence is required at quality %q", v.quality)
	}
	for i := range evidence {
		evidence[i].validate(v, index(path, i), methods)
	}
	unique(v, path, evidence)
}
