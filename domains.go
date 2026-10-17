package mibig

import (
	"encoding/json"
	"slices"
	"strings"
)

// DomainType identifies the kind of a protein domain.
type DomainType string

const (
	DomainAcyltransferase   DomainType = "acyltransferase"
	DomainAdenylation       DomainType = "adenylation"
	DomainAminotransferase  DomainType = "aminotransferase"
	DomainAMPBinding        DomainType = "amp-binding"
	DomainBranching         DomainType = "branching"
	DomainCarrier           DomainType = "carrier"
	DomainCondensation      DomainType = "condensation"
	DomainCyclase           DomainType = "cyclase"
	DomainDehydratase       DomainType = "dehydratase"
	DomainEnoylreductase    DomainType = "enoylreductase"
	DomainEpimerase         DomainType = "epimerase"
	DomainHydroxylase       DomainType = "hydroxylase"
	DomainKetoreductase     DomainType = "ketoreductase"
	DomainKetosynthase      DomainType = "ketosynthase"
	DomainLigase            DomainType = "ligase"
	DomainMethyltransferase DomainType = "methyltransferase"
	DomainOther             DomainType = "other"
	DomainOxidase           DomainType = "oxidase"
	DomainProductTemplate   DomainType = "product_template"
	DomainThioesterase      DomainType = "thioesterase"
	DomainThioreductase     DomainType = "thioreductase"
)

// Domain is a protein domain of a biosynthetic enzyme. It is implemented by
// one *<Type>Domain struct per domain type, e.g. *AdenylationDomain, and by
// *UnknownDomain for unrecognised "type" values; there are no other
// implementations.
type Domain interface {
	// DomainType returns the domain type, as written in the "type" key.
	DomainType() DomainType
	// Base returns the fields common to all domain types.
	Base() *DomainBase
	validate(v *validator, path string)
}

// Domains is a list of domains. It decodes each element into the right
// Domain implementation.
type Domains []Domain

// domainUnion lists the domain variants. The "type" value of each variant is
// the one its DomainType method returns.
var domainUnion = newUnion(unionSpec[Domain, DomainType]{
	key:   "type",
	tagOf: Domain.DomainType,
	variants: []func() Domain{
		func() Domain { return &AcyltransferaseDomain{} },
		func() Domain { return &AdenylationDomain{} },
		func() Domain { return &AminotransferaseDomain{} },
		func() Domain { return &AMPBindingDomain{} },
		func() Domain { return &BranchingDomain{} },
		func() Domain { return &CarrierDomain{} },
		func() Domain { return &CondensationDomain{} },
		func() Domain { return &CyclaseDomain{} },
		func() Domain { return &DehydrataseDomain{} },
		func() Domain { return &EnoylreductaseDomain{} },
		func() Domain { return &EpimeraseDomain{} },
		func() Domain { return &HydroxylaseDomain{} },
		func() Domain { return &KetoreductaseDomain{} },
		func() Domain { return &KetosynthaseDomain{} },
		func() Domain { return &LigaseDomain{} },
		func() Domain { return &MethyltransferaseDomain{} },
		func() Domain { return &OtherDomain{} },
		func() Domain { return &OxidaseDomain{} },
		func() Domain { return &ProductTemplateDomain{} },
		func() Domain { return &ThioesteraseDomain{} },
		func() Domain { return &ThioreductaseDomain{} },
	},
	unknown: func(domainType DomainType, raw json.RawMessage) Domain {
		d := &UnknownDomain{Type: domainType, Raw: raw}
		var probe struct {
			DomainBase
			Extra Extras `json:"-"`
		}
		if decodeObject(raw, &probe) == nil {
			d.DomainBase = probe.DomainBase
		}
		return d
	},
})

// UnmarshalJSON decodes a list of domains.
func (d *Domains) UnmarshalJSON(data []byte) error {
	list, err := domainUnion.decodeList(data)
	*d = list
	return err
}

// validate validates the domains. Domains of the types in notAllowed are
// reported.
func (d Domains) validate(v *validator, path string, notAllowed []DomainType) {
	for i, domain := range d {
		if isNil(domain) {
			v.errorf(index(path, i), CodeRequired, "domain must not be null")
			continue
		}
		if slices.Contains(notAllowed, domain.DomainType()) {
			v.errorf(join(index(path, i), "type"), CodeNotAllowed, "%q domains are not allowed here", domain.DomainType())
		}
		domain.validate(v, index(path, i))
	}
	unique(v, path, d)
}

// DomainBase holds the fields common to all domain types.
type DomainBase struct {
	Gene     GeneID   `json:"gene"`
	Location Location `json:"location"`
}

// Base returns the domain base, so that embedding types implement Domain.
func (b *DomainBase) Base() *DomainBase {
	return b
}

func (b *DomainBase) validate(v *validator, path string) {
	b.Gene.validate(v, join(path, "gene"))
	b.Location.validate(v, join(path, "location"))
}

// AcyltransferaseDomain is an acyltransferase (AT) domain.
type AcyltransferaseDomain struct {
	DomainBase
	// Subtype is "cis-AT" or "trans-AT".
	Subtype    string                     `json:"subtype,omitzero"`
	Substrates []AcyltransferaseSubstrate `json:"substrates,omitzero" mibig:"nonempty"`
	Evidence   []Evidence                 `json:"evidence,omitzero" mibig:"nonempty"`
	Inactive   *bool                      `json:"inactive,omitzero"`
	Extra      Extras                     `json:"-"`
}

func (d *AcyltransferaseDomain) DomainType() DomainType { return DomainAcyltransferase }

func (d *AcyltransferaseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.optionalOneOf(join(path, "subtype"), d.Subtype, acyltransferaseSubtypes, "acyltransferase subtype")
	for i := range d.Substrates {
		d.Substrates[i].validate(v, index(join(path, "substrates"), i))
	}
	unique(v, join(path, "substrates"), d.Substrates)
	validateEvidence(v, join(path, "evidence"), d.Evidence, substrateEvidenceMethods, len(d.Substrates) > 0)
	validateInactive(v, path, d.Inactive, len(d.Substrates) > 0, len(d.Evidence) > 0)
}

// validateInactive checks the rules for domains marked as inactive: they
// cannot have substrates and need evidence for being inactive.
func validateInactive(v *validator, path string, inactive *bool, hasSubstrates, hasEvidence bool) {
	if inactive == nil || !*inactive {
		return
	}
	if hasSubstrates {
		v.errorf(join(path, "substrates"), CodeInconsistent, "inactive domains cannot have substrates")
	}
	if !hasEvidence {
		v.errorf(join(path, "evidence"), CodeMissingEvidence, "inactive domains need evidence")
	}
}

// AcyltransferaseSubstrate is a substrate of an acyltransferase domain.
type AcyltransferaseSubstrate struct {
	// Name is e.g. "malonyl-CoA", or "other" with Details and Structure.
	Name      string `json:"name"`
	Structure SMILES `json:"structure,omitzero"`
	Details   string `json:"details,omitzero"`
	Extra     Extras `json:"-"`
}

func (s *AcyltransferaseSubstrate) validate(v *validator, path string) {
	v.oneOf(join(path, "name"), s.Name, acyltransferaseSubstrates, "acyltransferase substrate")
	if s.Name == "other" {
		v.required(join(path, "details"), s.Details, `details for substrate "other"`)
		if s.Structure == "" && v.strict() {
			v.errorf(join(path, "structure"), CodeRequired, `a structure is required for substrate "other"`)
		}
	}
	if s.Structure != "" {
		s.Structure.validate(v, join(path, "structure"))
	}
}

// AdenylationDomain is an adenylation (A) domain.
type AdenylationDomain struct {
	DomainBase
	// References is not part of the schema.
	References            []Citation             `json:"references,omitzero"`
	Substrates            []AdenylationSubstrate `json:"substrates,omitzero" mibig:"nonempty"`
	Evidence              []Evidence             `json:"evidence,omitzero" mibig:"nonempty"`
	PrecursorBiosynthesis []GeneID               `json:"precursor_biosynthesis,omitzero" mibig:"nonempty"`
	Inactive              *bool                  `json:"inactive,omitzero"`
	Extra                 Extras                 `json:"-"`
}

func (d *AdenylationDomain) DomainType() DomainType { return DomainAdenylation }

func (d *AdenylationDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	validateAdenylation(v, path, d.References, d.Substrates, d.Evidence, d.PrecursorBiosynthesis, d.Inactive)
}

func validateAdenylation(v *validator, path string, references []Citation, substrates []AdenylationSubstrate,
	evidence []Evidence, precursorBiosynthesis []GeneID, inactive *bool) {
	v.citations(join(path, "references"), references, false)
	for i := range substrates {
		substrates[i].validate(v, index(join(path, "substrates"), i))
	}
	unique(v, join(path, "substrates"), substrates)
	validateEvidence(v, join(path, "evidence"), evidence, substrateEvidenceMethods, len(substrates) > 0)
	v.geneIDs(join(path, "precursor_biosynthesis"), precursorBiosynthesis)
	validateInactive(v, path, inactive, len(substrates) > 0, len(evidence) > 0)
}

// AdenylationSubstrate is a substrate of an adenylation domain.
type AdenylationSubstrate struct {
	Name      string `json:"name"`
	Structure SMILES `json:"structure"`
	// Proteinogenic marks the 20 proteinogenic amino acids. Their names must
	// be the lower case amino acid name, e.g. "aspartic acid".
	Proteinogenic *bool  `json:"proteinogenic"`
	Extra         Extras `json:"-"`
}

func (s *AdenylationSubstrate) validate(v *validator, path string) {
	v.required(join(path, "name"), s.Name, "substrate name")
	s.Structure.validate(v, join(path, "structure"))
	v.requiredBool(join(path, "proteinogenic"), s.Proteinogenic, "proteinogenic")
	if s.Proteinogenic != nil && *s.Proteinogenic && !proteinogenicAminoAcids.has(strings.ToLower(s.Name)) {
		v.errorf(join(path, "name"), CodeInvalidValue, "%q is not a proteinogenic amino acid", s.Name)
	}
}

// AMPBindingDomain is an AMP-binding domain, the generic form of an
// adenylation domain. Not part of the schema.
type AMPBindingDomain struct {
	DomainBase
	References            []Citation             `json:"references,omitzero"`
	Substrates            []AdenylationSubstrate `json:"substrates,omitzero" mibig:"nonempty"`
	Evidence              []Evidence             `json:"evidence,omitzero" mibig:"nonempty"`
	PrecursorBiosynthesis []GeneID               `json:"precursor_biosynthesis,omitzero" mibig:"nonempty"`
	Inactive              *bool                  `json:"inactive,omitzero"`
	Extra                 Extras                 `json:"-"`
}

func (d *AMPBindingDomain) DomainType() DomainType { return DomainAMPBinding }

func (d *AMPBindingDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	validateAdenylation(v, path, d.References, d.Substrates, d.Evidence, d.PrecursorBiosynthesis, d.Inactive)
}

// AminotransferaseDomain is an aminotransferase domain.
type AminotransferaseDomain struct {
	DomainBase
	Inactive *bool `json:"inactive,omitzero"`
	// References is not part of the schema.
	References []Citation `json:"references,omitzero"`
	Extra      Extras     `json:"-"`
}

func (d *AminotransferaseDomain) DomainType() DomainType { return DomainAminotransferase }

func (d *AminotransferaseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.citations(join(path, "references"), d.References, false)
	if d.Inactive != nil && *d.Inactive && len(d.References) == 0 {
		v.errorf(join(path, "references"), CodeMissingEvidence, "inactive domains need references")
	}
}

// CarrierDomain is a carrier protein (ACP or PCP) domain.
type CarrierDomain struct {
	DomainBase
	// Subtype is "ACP" or "PCP".
	Subtype       string     `json:"subtype,omitzero"`
	Inactive      *bool      `json:"inactive,omitzero"`
	BetaBranching *bool      `json:"beta_branching,omitzero"`
	Evidence      []Evidence `json:"evidence,omitzero" mibig:"nonempty"`
	Extra         Extras     `json:"-"`
}

func (d *CarrierDomain) DomainType() DomainType { return DomainCarrier }

func (d *CarrierDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.optionalOneOf(join(path, "subtype"), d.Subtype, carrierSubtypes, "carrier subtype")
	validateEvidence(v, join(path, "evidence"), d.Evidence, substrateEvidenceMethods, false)
}

// CondensationDomain is a condensation (C) domain.
type CondensationDomain struct {
	DomainBase
	Subtype    string     `json:"subtype,omitzero"`
	References []Citation `json:"references,omitzero" mibig:"nonempty"`
	Extra      Extras     `json:"-"`
}

func (d *CondensationDomain) DomainType() DomainType { return DomainCondensation }

func (d *CondensationDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.optionalOneOf(join(path, "subtype"), d.Subtype, condensationSubtypes, "condensation subtype")
	v.citations(join(path, "references"), d.References, d.Subtype != "")
}

// CyclaseDomain is a cyclase domain.
type CyclaseDomain struct {
	DomainBase
	References []Citation `json:"references,omitzero" mibig:"nonempty"`
	Extra      Extras     `json:"-"`
}

func (d *CyclaseDomain) DomainType() DomainType { return DomainCyclase }

func (d *CyclaseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.citations(join(path, "references"), d.References, false)
}

// KetoreductaseDomain is a ketoreductase (KR) domain.
type KetoreductaseDomain struct {
	DomainBase
	Evidence []Evidence `json:"evidence,omitzero" mibig:"nonempty"`
	Inactive *bool      `json:"inactive,omitzero"`
	// Stereochemistry is the KR type, e.g. "A1" or "B2"; "A" or "B" if the
	// subtype is not known.
	Stereochemistry string `json:"stereochemistry,omitzero"`
	Extra           Extras `json:"-"`
}

func (d *KetoreductaseDomain) DomainType() DomainType { return DomainKetoreductase }

func (d *KetoreductaseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	validateEvidence(v, join(path, "evidence"), d.Evidence, substrateEvidenceMethods, true)
	v.optionalOneOf(join(path, "stereochemistry"), d.Stereochemistry, ketoreductaseStereochemistry, "ketoreductase stereochemistry")
}

// LigaseDomain is a ligase domain.
type LigaseDomain struct {
	DomainBase
	// Substrates is a single "substrate" in the schema.
	Substrates []SMILES `json:"substrates,omitzero" mibig:"alias=substrate,single"`
	// Evidence is a single object in the schema.
	Evidence []Evidence `json:"evidence,omitzero" mibig:"single"`
	Extra    Extras     `json:"-"`
}

func (d *LigaseDomain) DomainType() DomainType { return DomainLigase }

func (d *LigaseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	for i, substrate := range d.Substrates {
		substrate.validate(v, index(join(path, "substrates"), i))
	}
	unique(v, join(path, "substrates"), d.Substrates)
	validateEvidence(v, join(path, "evidence"), d.Evidence, substrateEvidenceMethods, len(d.Substrates) > 0)
}

// MethyltransferaseDomain is a methyltransferase (MT) domain.
type MethyltransferaseDomain struct {
	DomainBase
	// Subtype is "C", "N", "O", or "other" with Details.
	Subtype string `json:"subtype,omitzero"`
	Details string `json:"details,omitzero"`
	Extra   Extras `json:"-"`
}

func (d *MethyltransferaseDomain) DomainType() DomainType { return DomainMethyltransferase }

func (d *MethyltransferaseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.optionalOneOf(join(path, "subtype"), d.Subtype, methyltransferaseSubtypes, "methyltransferase subtype")
	if d.Subtype == "other" {
		v.required(join(path, "details"), d.Details, `details for subtype "other"`)
	}
}

// OtherDomain is a domain of a type without a domain type of its own.
type OtherDomain struct {
	DomainBase
	// Subtype names the domain, e.g. "Pyran synthase".
	Subtype string `json:"subtype"`
	// Active is not part of the schema.
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

func (d *OtherDomain) DomainType() DomainType { return DomainOther }

func (d *OtherDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.required(join(path, "subtype"), d.Subtype, "domain subtype")
}

// ThioesteraseDomain is a thioesterase (TE) domain.
type ThioesteraseDomain struct {
	DomainBase
	// Subtype is "Type I" or "Type II".
	Subtype string `json:"subtype,omitzero"`
	Extra   Extras `json:"-"`
}

func (d *ThioesteraseDomain) DomainType() DomainType { return DomainThioesterase }

func (d *ThioesteraseDomain) validate(v *validator, path string) {
	d.DomainBase.validate(v, path)
	v.optionalOneOf(join(path, "subtype"), d.Subtype, thioesteraseSubtypes, "thioesterase subtype")
}

// The following domain types have no fields of their own. Active is not part
// of the schema.

// BranchingDomain is a beta-branching domain. Not part of the schema.
type BranchingDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// DehydrataseDomain is a dehydratase (DH) domain.
type DehydrataseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// EnoylreductaseDomain is an enoylreductase (ER) domain.
type EnoylreductaseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// EpimeraseDomain is an epimerase (E) domain.
type EpimeraseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// HydroxylaseDomain is a hydroxylase domain.
type HydroxylaseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// KetosynthaseDomain is a ketosynthase (KS) domain.
type KetosynthaseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// OxidaseDomain is an oxidase domain.
type OxidaseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// ProductTemplateDomain is a product template (PT) domain. Not part of the
// schema.
type ProductTemplateDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

// ThioreductaseDomain is a thioreductase domain.
type ThioreductaseDomain struct {
	DomainBase
	Active *bool  `json:"active,omitzero"`
	Extra  Extras `json:"-"`
}

func (d *BranchingDomain) DomainType() DomainType       { return DomainBranching }
func (d *DehydrataseDomain) DomainType() DomainType     { return DomainDehydratase }
func (d *EnoylreductaseDomain) DomainType() DomainType  { return DomainEnoylreductase }
func (d *EpimeraseDomain) DomainType() DomainType       { return DomainEpimerase }
func (d *HydroxylaseDomain) DomainType() DomainType     { return DomainHydroxylase }
func (d *KetosynthaseDomain) DomainType() DomainType    { return DomainKetosynthase }
func (d *OxidaseDomain) DomainType() DomainType         { return DomainOxidase }
func (d *ProductTemplateDomain) DomainType() DomainType { return DomainProductTemplate }
func (d *ThioreductaseDomain) DomainType() DomainType   { return DomainThioreductase }

func (d *BranchingDomain) validate(v *validator, path string)       { d.DomainBase.validate(v, path) }
func (d *DehydrataseDomain) validate(v *validator, path string)     { d.DomainBase.validate(v, path) }
func (d *EnoylreductaseDomain) validate(v *validator, path string)  { d.DomainBase.validate(v, path) }
func (d *EpimeraseDomain) validate(v *validator, path string)       { d.DomainBase.validate(v, path) }
func (d *HydroxylaseDomain) validate(v *validator, path string)     { d.DomainBase.validate(v, path) }
func (d *KetosynthaseDomain) validate(v *validator, path string)    { d.DomainBase.validate(v, path) }
func (d *OxidaseDomain) validate(v *validator, path string)         { d.DomainBase.validate(v, path) }
func (d *ProductTemplateDomain) validate(v *validator, path string) { d.DomainBase.validate(v, path) }
func (d *ThioreductaseDomain) validate(v *validator, path string)   { d.DomainBase.validate(v, path) }

// UnknownDomain holds a domain with an unrecognised "type" value. Raw is the
// original JSON, which is written back unchanged; DomainBase holds the common
// fields as far as they could be decoded, for information only.
type UnknownDomain struct {
	DomainBase
	Type DomainType
	Raw  json.RawMessage
}

func (d *UnknownDomain) DomainType() DomainType { return d.Type }

// MarshalJSON writes the original JSON.
func (d UnknownDomain) MarshalJSON() ([]byte, error) {
	return rawOrTag(d.Raw, "type", string(d.Type))
}

func (d *UnknownDomain) validate(v *validator, path string) {
	v.errorf(join(path, "type"), CodeUnknownType, "unknown domain type %q", d.Type)
}
