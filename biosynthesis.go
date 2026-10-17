package mibig

// Biosynthesis describes how the compounds are made: the biosynthetic
// classes, the modules of modular enzymes, operons and biosynthetic paths.
type Biosynthesis struct {
	Classes Classes  `json:"classes"`
	Modules Modules  `json:"modules,omitzero" mibig:"nonempty"`
	Operons []Operon `json:"operons,omitzero" mibig:"nonempty"`
	// Paths are required by the schema, but the requirement is not enforced
	// yet, as most entries do not have them.
	Paths []Path `json:"paths,omitzero" mibig:"nonempty"`
	Extra Extras `json:"-"`
}

func (b *Biosynthesis) validate(v *validator, path string) {
	requiredList(v, join(path, "classes"), b.Classes, "biosynthetic class")
	b.Classes.validate(v, join(path, "classes"))
	b.Modules.validate(v, join(path, "modules"))
	for i := range b.Operons {
		b.Operons[i].validate(v, index(join(path, "operons"), i))
	}
	unique(v, join(path, "operons"), b.Operons)
	for i := range b.Paths {
		b.Paths[i].validate(v, index(join(path, "paths"), i))
	}
	unique(v, join(path, "paths"), b.Paths)
}

// Operon is a group of co-transcribed genes.
type Operon struct {
	Genes    []GeneID   `json:"genes"`
	Evidence []Evidence `json:"evidence"`
	Extra    Extras     `json:"-"`
}

func (o *Operon) validate(v *validator, path string) {
	requiredList(v, join(path, "genes"), o.Genes, "gene")
	for i, gene := range o.Genes {
		gene.validate(v, index(join(path, "genes"), i))
	}
	validateEvidence(v, join(path, "evidence"), o.Evidence, operonEvidenceMethods, true)
}

// Path is a biosynthetic path leading to one or more products.
type Path struct {
	Products []PathProduct `json:"products"`
	// Steps describes the order of the biosynthetic steps, e.g.
	// "geneA > geneB, geneC > [module1]". The format is not fixed yet, so it
	// is kept as text.
	Steps             string     `json:"steps"`
	References        []Citation `json:"references"`
	IsSubcluster      *bool      `json:"isSubcluster"`
	ProducesPrecursor *bool      `json:"producesPrecursor"`
	Extra             Extras     `json:"-"`
}

func (p *Path) validate(v *validator, path string) {
	requiredList(v, join(path, "products"), p.Products, "product")
	for i := range p.Products {
		p.Products[i].validate(v, index(join(path, "products"), i))
	}
	unique(v, join(path, "products"), p.Products)
	v.required(join(path, "steps"), p.Steps, "steps")
	v.citations(join(path, "references"), p.References, true)
	v.requiredBool(join(path, "isSubcluster"), p.IsSubcluster, "isSubcluster")
	v.requiredBool(join(path, "producesPrecursor"), p.ProducesPrecursor, "producesPrecursor")
}

// PathProduct is an intermediate or final product of a biosynthetic path.
type PathProduct struct {
	Name      string `json:"name"`
	Structure SMILES `json:"structure,omitzero"`
	Comment   string `json:"comment,omitzero"`
	Extra     Extras `json:"-"`
}

func (p *PathProduct) validate(v *validator, path string) {
	v.required(join(path, "name"), p.Name, "product name")
	if p.Structure != "" {
		p.Structure.validate(v, join(path, "structure"))
	}
}

// Monomer is a building block incorporated by a module, or the starter unit
// of a PKS.
//
// The schema has two monomer definitions: one with references (used by
// modules) and one with evidence (defined, but not used). The data uses
// both fields, so Monomer has both.
type Monomer struct {
	Name       string     `json:"name"`
	Structure  SMILES     `json:"structure"`
	Evidence   []Evidence `json:"evidence,omitzero" mibig:"nonempty"`
	References []Citation `json:"references,omitzero" mibig:"nonempty"`
	Extra      Extras     `json:"-"`
}

func (m *Monomer) validate(v *validator, path string) {
	v.required(join(path, "name"), m.Name, "monomer name")
	m.Structure.validate(v, join(path, "structure"))
	if len(m.Evidence) == 0 && len(m.References) == 0 && v.strict() {
		v.errorf(path, CodeMissingEvidence, "monomers need evidence or references at quality %q", v.quality)
	}
	validateEvidence(v, join(path, "evidence"), m.Evidence, substrateEvidenceMethods, false)
	v.citations(join(path, "references"), m.References, false)
}

// ReleaseType describes how the product is released from an NRPS.
// References are optional, but always written, even if empty, like the data
// does.
type ReleaseType struct {
	Name       string     `json:"name"`
	Details    string     `json:"details,omitzero"`
	References []Citation `json:"references" mibig:"optional"`
	Extra      Extras     `json:"-"`
}

func (r *ReleaseType) validate(v *validator, path string) {
	v.oneOf(join(path, "name"), r.Name, releaseTypeNames, "release type")
	if r.Name == "Other" {
		v.required(join(path, "details"), r.Details, `details for release type "Other"`)
	}
	v.citations(join(path, "references"), r.References, false)
}
