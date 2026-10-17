package mibig

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
)

// ClassName identifies a biosynthetic class.
type ClassName string

const (
	ClassNRPS       ClassName = "NRPS"
	ClassPKS        ClassName = "PKS"
	ClassRibosomal  ClassName = "ribosomal"
	ClassSaccharide ClassName = "saccharide"
	ClassTerpene    ClassName = "terpene"
	ClassOther      ClassName = "other"
)

// Class is a biosynthetic class of a gene cluster. It is implemented by
// *NRPSClass, *PKSClass, *RibosomalClass, *SaccharideClass, *TerpeneClass,
// *OtherClass, and *UnknownClass for unrecognised "class" values; there are no
// other implementations.
type Class interface {
	// ClassName returns the class, as written in the "class" key.
	ClassName() ClassName
	validate(v *validator, path string)
}

// Classes is a list of biosynthetic classes. It decodes each element into
// the right Class implementation.
type Classes []Class

// classUnion lists the class variants. The "class" value of each variant is
// the one its ClassName method returns.
var classUnion = newUnion(unionSpec[Class, ClassName]{
	key:   "class",
	tagOf: Class.ClassName,
	variants: []func() Class{
		func() Class { return &NRPSClass{} },
		func() Class { return &PKSClass{} },
		func() Class { return &RibosomalClass{} },
		func() Class { return &SaccharideClass{} },
		func() Class { return &TerpeneClass{} },
		func() Class { return &OtherClass{} },
	},
	aliases: map[string]ClassName{"pks": ClassPKS},
	unknown: func(name ClassName, raw json.RawMessage) Class {
		return &UnknownClass{Name: name, Raw: raw}
	},
})

// UnmarshalJSON decodes a list of classes.
func (c *Classes) UnmarshalJSON(data []byte) error {
	list, err := classUnion.decodeList(data)
	*c = list
	return err
}

func (c Classes) validate(v *validator, path string) {
	for i, class := range c {
		if isNil(class) {
			v.errorf(index(path, i), CodeRequired, "class must not be null")
			continue
		}
		class.validate(v, index(path, i))
	}
	unique(v, path, c)
}

// isNil reports whether an interface value is nil or holds a nil pointer.
func isNil(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// NRPSClass describes a nonribosomal peptide synthetase.
type NRPSClass struct {
	Subclass      string               `json:"subclass"`
	Thioesterases []ThioesteraseDomain `json:"thioesterases,omitzero"`
	// ReleaseTypes is called "release_type" in the schema.
	ReleaseTypes []ReleaseType `json:"release_types,omitzero" mibig:"alias=release_type"`
	Extra        Extras        `json:"-"`
}

func (c *NRPSClass) ClassName() ClassName { return ClassNRPS }

func (c *NRPSClass) validate(v *validator, path string) {
	if v.strict() {
		v.oneOf(join(path, "subclass"), c.Subclass, nrpsSubclasses, "NRPS subclass")
	}
	for i := range c.ReleaseTypes {
		c.ReleaseTypes[i].validate(v, index(join(path, "release_types"), i))
	}
	unique(v, join(path, "release_types"), c.ReleaseTypes)
	for i := range c.Thioesterases {
		c.Thioesterases[i].validate(v, index(join(path, "thioesterases"), i))
	}
	unique(v, join(path, "thioesterases"), c.Thioesterases)
}

// PKSClass describes a polyketide synthase.
type PKSClass struct {
	Subclass    string   `json:"subclass"`
	Cyclases    []GeneID `json:"cyclases,omitzero" mibig:"nonempty"`
	StarterUnit *Monomer `json:"starter_unit,omitzero"`
	// KetideLength is the number of ketide units.
	KetideLength *int `json:"ketide_length,omitzero"`
	// Iterative is a placeholder: the schema defines an empty object, the
	// Python implementation a boolean, and the data does not use it.
	Iterative     json.RawMessage      `json:"iterative,omitzero"`
	Thioesterases []ThioesteraseDomain `json:"thioesterases,omitzero"`
	Extra         Extras               `json:"-"`
}

func (c *PKSClass) ClassName() ClassName { return ClassPKS }

func (c *PKSClass) validate(v *validator, path string) {
	if v.strict() {
		v.oneOf(join(path, "subclass"), c.Subclass, pksSubclasses, "PKS subclass")
	}
	v.geneIDs(join(path, "cyclases"), c.Cyclases)
	if c.StarterUnit != nil {
		c.StarterUnit.validate(v, join(path, "starter_unit"))
	}
	if c.KetideLength != nil && *c.KetideLength < 0 {
		v.errorf(join(path, "ketide_length"), CodeInvalidValue, "ketide length must not be negative, got %d", *c.KetideLength)
	}
	for i := range c.Thioesterases {
		c.Thioesterases[i].validate(v, index(join(path, "thioesterases"), i))
	}
	unique(v, join(path, "thioesterases"), c.Thioesterases)
}

// RibosomalClass describes a ribosomally synthesised peptide.
type RibosomalClass struct {
	// Subclass is "RiPP" or "unmodified".
	Subclass   string          `json:"subclass"`
	Precursors []RiPPPrecursor `json:"precursors"`
	RiPPType   string          `json:"ripp_type,omitzero"`
	Details    string          `json:"details,omitzero"`
	Peptidases []GeneID        `json:"peptidases,omitzero" mibig:"nonempty"`
	Extra      Extras          `json:"-"`
}

func (c *RibosomalClass) ClassName() ClassName { return ClassRibosomal }

func (c *RibosomalClass) validate(v *validator, path string) {
	v.oneOf(join(path, "subclass"), c.Subclass, ribosomalSubclasses, "ribosomal subclass")
	if c.Subclass == "RiPP" && v.strict() {
		v.oneOf(join(path, "ripp_type"), c.RiPPType, rippTypes, "RiPP type")
	}
	if c.RiPPType == "other" || c.RiPPType == "Other" {
		v.required(join(path, "details"), c.Details, `details for RiPP type "other"`)
	}
	if len(c.Precursors) == 0 && v.atLeast(QualityMedium) {
		v.errorf(join(path, "precursors"), CodeRequired, "at least one precursor is required at quality %q", v.quality)
	}
	for i := range c.Precursors {
		c.Precursors[i].validate(v, index(join(path, "precursors"), i))
	}
	unique(v, join(path, "precursors"), c.Precursors)
	v.geneIDs(join(path, "peptidases"), c.Peptidases)
}

// RiPPPrecursor is the precursor peptide of a RiPP.
type RiPPPrecursor struct {
	Gene         GeneID       `json:"gene"`
	CoreSequence CoreSequence `json:"core_sequence"`
	// Cleavage locations are in amino acid coordinates of the precursor.
	LeaderCleavageLocation   *Location   `json:"leader_cleavage_location,omitzero"`
	FollowerCleavageLocation *Location   `json:"follower_cleavage_location,omitzero"`
	Crosslinks               []Crosslink `json:"crosslinks,omitzero" mibig:"nonempty"`
	RecognitionMotif         string      `json:"recognition_motif,omitzero"`
	Extra                    Extras      `json:"-"`
}

func (p *RiPPPrecursor) validate(v *validator, path string) {
	p.Gene.validate(v, join(path, "gene"))
	p.CoreSequence.validate(v, join(path, "core_sequence"))
	for i := range p.Crosslinks {
		p.Crosslinks[i].validate(v, index(join(path, "crosslinks"), i))
	}
	unique(v, join(path, "crosslinks"), p.Crosslinks)
	if p.LeaderCleavageLocation != nil {
		p.LeaderCleavageLocation.validate(v, join(path, "leader_cleavage_location"))
	}
	if p.FollowerCleavageLocation != nil {
		p.FollowerCleavageLocation.validate(v, join(path, "follower_cleavage_location"))
	}
}

// CoreSequence is the core peptide sequence of a RiPP precursor.
//
// Current entries store a single sequence as a string. Some older entries
// store a list of sequences; Legacy records that, so the value is written
// back the same way. The validator warns about the legacy form.
type CoreSequence struct {
	Sequences []string
	Legacy    bool
}

// IsZero reports whether there is no sequence.
func (c CoreSequence) IsZero() bool {
	return len(c.Sequences) == 0 && !c.Legacy
}

// MarshalJSON writes a single sequence as a string, and a list otherwise.
func (c CoreSequence) MarshalJSON() ([]byte, error) {
	if !c.Legacy && len(c.Sequences) <= 1 {
		sequence := ""
		if len(c.Sequences) == 1 {
			sequence = c.Sequences[0]
		}
		return marshalNoEscape(sequence)
	}
	if c.Sequences == nil {
		return []byte("[]"), nil
	}
	return marshalNoEscape(c.Sequences)
}

// UnmarshalJSON reads both the string and the legacy list form.
func (c *CoreSequence) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return err
		}
		sequences := make([]string, len(items))
		for i, item := range items {
			if isNull(item) {
				return wrapIndex(i, errors.New("null is not allowed in this list"))
			}
			if err := json.Unmarshal(item, &sequences[i]); err != nil {
				return wrapIndex(i, err)
			}
		}
		*c = CoreSequence{Sequences: sequences, Legacy: true}
		return nil
	}
	var sequence string
	if err := json.Unmarshal(trimmed, &sequence); err != nil {
		return err
	}
	*c = CoreSequence{}
	if sequence != "" {
		c.Sequences = []string{sequence}
	}
	return nil
}

func (c *CoreSequence) validate(v *validator, path string) {
	if len(c.Sequences) == 0 {
		v.errorf(path, CodeRequired, "core sequence is required")
	}
	for _, sequence := range c.Sequences {
		if sequence == "" {
			v.errorf(path, CodeRequired, "core sequence must not be empty")
		}
	}
	if c.Legacy || len(c.Sequences) > 1 {
		v.warnf(path, CodeLegacyFormat, "core sequence is stored as a list, use a single string")
	}
}

// Crosslink is a crosslink between two residues of a RiPP core peptide.
type Crosslink struct {
	From int `json:"from"`
	To   int `json:"to"`
	// Type is "ether", "thioether" or "other".
	Type    string `json:"type,omitzero"`
	Details string `json:"details,omitzero"`
	Extra   Extras `json:"-"`
}

func (c *Crosslink) validate(v *validator, path string) {
	if c.From < 1 {
		v.errorf(join(path, "from"), CodeInvalidValue, "crosslink start must be at least 1, got %d", c.From)
	}
	if c.To < 2 {
		v.errorf(join(path, "to"), CodeInvalidValue, "crosslink end must be at least 2, got %d", c.To)
	}
	if c.From >= c.To {
		v.errorf(path, CodeInconsistent, "crosslink start %d must be before end %d", c.From, c.To)
	}
	v.optionalOneOf(join(path, "type"), c.Type, crosslinkTypes, "crosslink type")
}

// SaccharideClass describes saccharide biosynthesis.
type SaccharideClass struct {
	Glycosyltransferases []Glycosyltransferase `json:"glycosyltransferases"`
	// Subclass has no controlled vocabulary yet.
	Subclass    string       `json:"subclass,omitzero"`
	Subclusters []Subcluster `json:"subclusters,omitzero" mibig:"nonempty"`
	Extra       Extras       `json:"-"`
}

func (c *SaccharideClass) ClassName() ClassName { return ClassSaccharide }

func (c *SaccharideClass) validate(v *validator, path string) {
	if v.strict() {
		requiredList(v, join(path, "glycosyltransferases"), c.Glycosyltransferases, "glycosyltransferase")
	}
	for i := range c.Glycosyltransferases {
		c.Glycosyltransferases[i].validate(v, index(join(path, "glycosyltransferases"), i))
	}
	unique(v, join(path, "glycosyltransferases"), c.Glycosyltransferases)
	for i := range c.Subclusters {
		c.Subclusters[i].validate(v, index(join(path, "subclusters"), i))
	}
	unique(v, join(path, "subclusters"), c.Subclusters)
}

// Glycosyltransferase is a glycosyltransferase gene and its specificity.
type Glycosyltransferase struct {
	Gene GeneID `json:"gene"`
	// Evidence is a single object in the schema and a list in the data.
	Evidence []Evidence `json:"evidence" mibig:"single"`
	// Specificity is the transferred sugar.
	Specificity SMILES `json:"specificity,omitzero"`
	Extra       Extras `json:"-"`
}

func (g *Glycosyltransferase) validate(v *validator, path string) {
	g.Gene.validate(v, join(path, "gene"))
	validateEvidence(v, join(path, "evidence"), g.Evidence, glycosyltransferaseEvidenceMethods, true)
	if g.Specificity != "" {
		g.Specificity.validate(v, join(path, "specificity"))
	}
}

// Subcluster is a group of genes making a building block, e.g. a sugar.
type Subcluster struct {
	Genes       []GeneID   `json:"genes"`
	References  []Citation `json:"references"`
	Specificity SMILES     `json:"specificity,omitzero"`
	Extra       Extras     `json:"-"`
}

func (s *Subcluster) validate(v *validator, path string) {
	requiredList(v, join(path, "genes"), s.Genes, "gene")
	v.geneIDs(join(path, "genes"), s.Genes)
	v.citations(join(path, "references"), s.References, true)
	if s.Specificity != "" {
		s.Specificity.validate(v, join(path, "specificity"))
	}
}

// TerpeneClass describes terpene biosynthesis.
type TerpeneClass struct {
	Subclass           string   `json:"subclass"`
	Prenyltransferases []GeneID `json:"prenyltransferases,omitzero" mibig:"nonempty"`
	SynthasesCyclases  []GeneID `json:"synthases_cyclases,omitzero" mibig:"nonempty"`
	// Synthases is the legacy name of SynthasesCyclases. The validator warns
	// about it.
	Synthases []GeneID `json:"synthases,omitzero" mibig:"nonempty"`
	// Precursor is "DMAPP", "FPP", "GGPP", "GPP" or "IPP".
	Precursor string `json:"precursor,omitzero"`
	Extra     Extras `json:"-"`
}

func (c *TerpeneClass) ClassName() ClassName { return ClassTerpene }

func (c *TerpeneClass) validate(v *validator, path string) {
	if v.strict() {
		v.oneOf(join(path, "subclass"), c.Subclass, terpeneSubclasses, "terpene subclass")
	}
	v.geneIDs(join(path, "prenyltransferases"), c.Prenyltransferases)
	v.geneIDs(join(path, "synthases_cyclases"), c.SynthasesCyclases)
	if len(c.Synthases) > 0 {
		v.warnf(join(path, "synthases"), CodeLegacyFormat, `"synthases" is the legacy name of "synthases_cyclases"`)
		v.geneIDs(join(path, "synthases"), c.Synthases)
	}
	v.optionalOneOf(join(path, "precursor"), c.Precursor, terpenePrecursors, "terpene precursor")
}

// OtherClass describes biosynthetic classes without a class of their own.
type OtherClass struct {
	Subclass string `json:"subclass"`
	Details  string `json:"details,omitzero"`
	Extra    Extras `json:"-"`
}

func (c *OtherClass) ClassName() ClassName { return ClassOther }

func (c *OtherClass) validate(v *validator, path string) {
	v.oneOf(join(path, "subclass"), c.Subclass, otherSubclasses, "subclass")
	if c.Subclass == "other" {
		v.required(join(path, "details"), c.Details, `details for subclass "other"`)
	}
}

// UnknownClass holds a class with an unrecognised "class" value. Raw is the
// original JSON, which is written back unchanged.
type UnknownClass struct {
	Name ClassName
	Raw  json.RawMessage
}

func (c *UnknownClass) ClassName() ClassName { return c.Name }

// MarshalJSON writes the original JSON.
func (c UnknownClass) MarshalJSON() ([]byte, error) {
	return rawOrTag(c.Raw, "class", string(c.Name))
}

// rawOrTag returns raw, or an object with just the discriminator if raw is
// empty, e.g. for an unknown variant created in Go.
func rawOrTag(raw json.RawMessage, key, tag string) ([]byte, error) {
	if len(raw) > 0 {
		return raw, nil
	}
	return marshalNoEscape(map[string]string{key: tag})
}

func (c *UnknownClass) validate(v *validator, path string) {
	v.errorf(join(path, "class"), CodeUnknownType, "unknown biosynthetic class %q", c.Name)
}
