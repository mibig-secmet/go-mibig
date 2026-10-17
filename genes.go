package mibig

import (
	"regexp"
	"strings"
)

// Genes holds gene level information: genes to add to or remove from the
// annotation of the locus, and functional annotations.
type Genes struct {
	ToAdd       []GeneAddition   `json:"to_add,omitzero" mibig:"nonempty"`
	ToDelete    []GeneDeletion   `json:"to_delete,omitzero" mibig:"nonempty"`
	Annotations []GeneAnnotation `json:"annotations,omitzero" mibig:"nonempty"`
	// Domains are protein domains, as defined by the schema. The data
	// annotates domains per gene instead, see GeneAnnotation.Domains.
	Domains []GeneDomain `json:"domains,omitzero" mibig:"nonempty"`
	Extra   Extras       `json:"-"`
}

func (g *Genes) validate(v *validator, path string) {
	for i := range g.ToAdd {
		g.ToAdd[i].validate(v, index(join(path, "to_add"), i))
	}
	unique(v, join(path, "to_add"), g.ToAdd)

	for i := range g.ToDelete {
		g.ToDelete[i].validate(v, index(join(path, "to_delete"), i))
	}
	unique(v, join(path, "to_delete"), g.ToDelete)

	for i := range g.Annotations {
		g.Annotations[i].validate(v, index(join(path, "annotations"), i))
	}
	unique(v, join(path, "annotations"), g.Annotations)

	for i := range g.Domains {
		g.Domains[i].validate(v, index(join(path, "domains"), i))
	}
	unique(v, join(path, "domains"), g.Domains)
}

// GeneAddition is a gene missing from the annotation of the locus.
type GeneAddition struct {
	ID       GeneID       `json:"id"`
	Location GeneLocation `json:"location"`
	// Translation is the protein sequence. It is written as null if empty.
	Translation *string `json:"translation" mibig:"optional"`
	Extra       Extras  `json:"-"`
}

func (a *GeneAddition) validate(v *validator, path string) {
	a.ID.validate(v, join(path, "id"))
	a.Location.validate(v, join(path, "location"))
	translationPath := join(path, "translation")
	if a.Translation == nil || *a.Translation == "" {
		if v.strict() {
			v.errorf(translationPath, CodeRequired, "a translation is required at quality %q", v.quality)
		}
		return
	}
	if i := strings.IndexFunc(*a.Translation, func(r rune) bool { return !strings.ContainsRune(aminoAcids, r) }); i >= 0 {
		v.errorf(translationPath, CodeInvalidValue, "invalid amino acid %q at position %d", (*a.Translation)[i:i+1], i+1)
	}
}

// GeneLocation is the location of a gene, as a list of exons on one strand.
type GeneLocation struct {
	Exons []Location `json:"exons"`
	// Strand is 1 for the forward and -1 for the reverse strand.
	Strand int    `json:"strand"`
	Extra  Extras `json:"-"`
}

func (l *GeneLocation) validate(v *validator, path string) {
	requiredList(v, join(path, "exons"), l.Exons, "exon")
	for i := range l.Exons {
		l.Exons[i].validate(v, index(join(path, "exons"), i))
	}
	unique(v, join(path, "exons"), l.Exons)
	if l.Strand != 1 && l.Strand != -1 {
		v.errorf(join(path, "strand"), CodeInvalidValue, "strand must be 1 or -1, got %d", l.Strand)
	}
}

// GeneDeletion is a gene to remove from the annotation of the locus.
type GeneDeletion struct {
	ID     GeneID `json:"id"`
	Reason string `json:"reason"`
	Extra  Extras `json:"-"`
}

func (d *GeneDeletion) validate(v *validator, path string) {
	d.ID.validate(v, join(path, "id"))
	v.required(join(path, "reason"), d.Reason, "deletion reason")
}

// GeneAnnotation annotates a gene of the locus.
type GeneAnnotation struct {
	ID        GeneID              `json:"id"`
	Name      GeneID              `json:"name,omitzero"`
	Aliases   []GeneID            `json:"aliases,omitzero" mibig:"nonempty"`
	Product   string              `json:"product,omitzero"`
	Functions []GeneFunction      `json:"functions,omitzero"`
	Tailoring []TailoringFunction `json:"tailoring,omitzero"`
	// The following fields are not part of the schema.
	Domains           Domains            `json:"domains,omitzero"`
	MutationPhenotype *MutationPhenotype `json:"mutation_phenotype,omitzero"`
	Comment           string             `json:"comment,omitzero"`
	Extra             Extras             `json:"-"`
}

func (a *GeneAnnotation) validate(v *validator, path string) {
	a.ID.validate(v, join(path, "id"))
	if a.Name != "" {
		a.Name.validate(v, join(path, "name"))
	}
	v.geneIDs(join(path, "aliases"), a.Aliases)
	for i := range a.Functions {
		a.Functions[i].validate(v, index(join(path, "functions"), i))
	}
	for i := range a.Tailoring {
		a.Tailoring[i].validate(v, index(join(path, "tailoring"), i))
	}
	unique(v, join(path, "tailoring"), a.Tailoring)
	a.Domains.validate(v, join(path, "domains"), nil)
	if a.MutationPhenotype != nil {
		a.MutationPhenotype.validate(v, join(path, "mutation_phenotype"))
	}
}

// GeneFunction is a function of a gene and the evidence for it.
type GeneFunction struct {
	Function          FunctionCategory   `json:"function"`
	Evidence          []Evidence         `json:"evidence"`
	MutationPhenotype *MutationPhenotype `json:"mutation_phenotype,omitzero"`
	Extra             Extras             `json:"-"`
}

func (f *GeneFunction) validate(v *validator, path string) {
	f.Function.validate(v, join(path, "function"))
	validateEvidence(v, join(path, "evidence"), f.Evidence, geneFunctionEvidenceMethods, true)
	if f.MutationPhenotype != nil {
		f.MutationPhenotype.validate(v, join(path, "mutation_phenotype"))
	}
}

// FunctionCategory is the category of a gene function, e.g. "Regulation".
type FunctionCategory struct {
	Name    string `json:"name"`
	Details string `json:"details,omitzero"`
	Extra   Extras `json:"-"`
}

func (c *FunctionCategory) validate(v *validator, path string) {
	v.oneOf(join(path, "name"), c.Name, geneFunctionNames, "gene function")
	if c.Name == "Other" {
		v.required(join(path, "details"), c.Details, `details for function "Other"`)
	}
}

// MutationPhenotype describes the phenotype of a gene knock-out.
type MutationPhenotype struct {
	Phenotype string `json:"phenotype"`
	// References is a single citation in the schema and a list in the data.
	References []Citation `json:"references" mibig:"single"`
	Details    string     `json:"details,omitzero"`
	Extra      Extras     `json:"-"`
}

func (m *MutationPhenotype) validate(v *validator, path string) {
	v.required(join(path, "phenotype"), m.Phenotype, "phenotype")
	v.citations(join(path, "references"), m.References, true)
}

// TailoringFunction is a tailoring reaction catalysed by a gene product.
type TailoringFunction struct {
	Function   string     `json:"function"`
	References []Citation `json:"references"`
	// DBReference is a MITE accession, e.g. "mite:MITE0000001".
	DBReference string `json:"db_reference,omitzero" mibig:"alias=dbReference"`
	Details     string `json:"details,omitzero"`
	Extra       Extras `json:"-"`
}

var miteReferencePattern = regexp.MustCompile(`^mite:MITE\d{7}$`)

func (t *TailoringFunction) validate(v *validator, path string) {
	v.oneOf(join(path, "function"), t.Function, tailoringFunctions, "tailoring function")
	v.citations(join(path, "references"), t.References, true)
	if t.DBReference != "" && !miteReferencePattern.MatchString(t.DBReference) {
		v.errorf(join(path, "db_reference"), CodeInvalidValue, "invalid MITE reference %q", t.DBReference)
	}
}

// GeneDomain is a protein domain of a gene, as defined by the schema.
type GeneDomain struct {
	Location   Location              `json:"location"`
	Name       string                `json:"name"`
	Substrates []GeneDomainSubstrate `json:"substrates,omitzero"`
	Extra      Extras                `json:"-"`
}

func (d *GeneDomain) validate(v *validator, path string) {
	d.Location.validate(v, join(path, "location"))
	v.required(join(path, "name"), d.Name, "domain name")
	for i := range d.Substrates {
		d.Substrates[i].validate(v, index(join(path, "substrates"), i))
	}
}

// GeneDomainSubstrate is a substrate of a GeneDomain.
type GeneDomainSubstrate struct {
	Evidence  []Evidence `json:"evidence"`
	Structure SMILES     `json:"structure"`
	Extra     Extras     `json:"-"`
}

func (s *GeneDomainSubstrate) validate(v *validator, path string) {
	validateEvidence(v, join(path, "evidence"), s.Evidence, substrateEvidenceMethods, true)
	s.Structure.validate(v, join(path, "structure"))
}
