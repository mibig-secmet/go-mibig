package mibig

// Controlled vocabularies. Unless noted otherwise, each list is the union of
// the values allowed by the JSON schema and by the Python implementation,
// which is ahead of the schema in several places. See docs/schema-notes.md.

var locusEvidenceMethods = vocabulary{
	"Homology-based prediction",
	"Correlation of genomic and metabolomic data",
	"Gene expression correlated with compound production",
	"Knock-out studies",
	"Enzymatic assays",
	"Heterologous expression",
	"In vitro expression",
	"Synthetic-bioinformatic natural product (syn-BNP)", // Python only
}

var substrateEvidenceMethods = vocabulary{
	"Activity assay",
	"ACVS assay",
	"ATP-PPi exchange assay",
	"Enzyme-coupled assay",
	"Feeding study",
	"Heterologous expression",
	"Homology",
	"HPLC",
	"In-vitro experiments",
	"Knock-out studies",
	"Mass spectrometry",
	"NMR",
	"Radio labelling",
	"Sequence-based prediction",
	"Steady-state kinetics",
	"Structure-based inference",
	"X-ray crystallography",
}

var compoundEvidenceMethods = vocabulary{
	"NMR",
	"Mass spectrometry",
	"MS/MS",
	"X-ray crystallography",
	"Chemical derivatisation",
	"Total synthesis",
	"Experimental values match with authentic standard", // Python only
	"Other", // data only, with details
}

// compoundClasses are the compound classes of the Python implementation. The
// schema only lists "alkaloid", "nucleoside" and "peptide", which the data
// does not use.
var compoundClasses = vocabulary{
	// Alkaloid
	"Amination reaction-derived",
	"Anthranilic acid-derived",
	"Arginine-derived",
	"Guanidine-derived",
	"Histidine-derived",
	"Lysine-derived",
	"Nicotinic acid-derived",
	"Ornithine-derived",
	"Peptide alkaloid",
	"Proline-derived",
	"Purine alkaloid",
	"Serine-derived",
	"Steroidal alkaloid",
	"Tetramate alkaloid",
	"Terpenoid-alkaloid",
	"Tryptophan-derived",
	"Tyrosine-derived",
	// Shikimic acid-derived
	"Aromatic amino acid/simple benzoic acid",
	"Aromatic polyketide",
	"Phenylpropanoid",
	"Terpenoid quinone",
	// Acetate-derived
	"Alkylresorcinol/phloroglucinol polyketide",
	"Chromane polyketide",
	"Cyclic polyketide",
	"Fatty acid",
	"Fatty acid derivate",
	"Linear polyketide",
	"Macrocyclic polyketide",
	"Naphthalene polyketide",
	"Polycyclic polyketide",
	"Polyether polyketide",
	"Xanthone polyketide",
	// Isoprene-derived
	"Atypical terpenoid",
	"Diterpenoid",
	"Hemiterpenoid",
	"Higher terpenoid",
	"Iridoid",
	"Meroterpenoid",
	"Monoterpenoid",
	"Sesquiterpenoid",
	"Sesterterpenoid",
	"Steroid",
	// Peptide
	"Beta-lactam",
	"Depsipeptide",
	"Diketopiperazine",
	"Glycopeptide",
	"Glycopeptidolipid",
	"Linear",
	"Lipopeptide",
	"Macrocyclic",
	// Carbohydrates
	"Monosaccharide",
	"Oligosaccharide",
	"Polysaccharide",
	"Nucleoside",
	"Aminoglycoside",
	"Liposaccharide",
	"Glucosinolate",
	// Glycolysis-derived
	"Butenolide",
	"Butyrolactone",
	"Tetronic acid",
	// Other
	"Lactone",
	"Ectoine",
	"Furan",
	"Phosphonate",
}

var geneFunctionNames = vocabulary{
	"Activation / processing",
	"Maturation",
	"Precursor",
	"Precursor biosynthesis",
	"Regulation",
	"Resistance/immunity",
	"Scaffold biosynthesis",
	"Tailoring",
	"Transport",
	"Other",
}

var geneFunctionEvidenceMethods = vocabulary{
	"Other in vivo study",
	"Heterologous expression",
	"Knock-out",
	"Activity assay",
}

var tailoringFunctions = vocabulary{
	"Acetylation",
	"Acylation",
	"Amination",
	"Biaryl bond formation",
	"Carboxylation",
	"Cyclization",
	"Deamination",
	"Decarboxylation",
	"Dehydration",
	"Dehydrogenation",
	"Demethylation",
	"Dioxygenation",
	"Epimerization",
	"FADH2 supply for chlorination",
	"Glycosylation",
	"Halogenation",
	"Heterocyclization",
	"Hydrolysis",
	"Hydroxylation",
	"Lasso macrolactam formation",
	"Macrolactam formation", // Python only
	"Methylation",
	"Monooxygenation",
	"Oxidation",
	"Phosphorylation",
	"Prenylation",
	"Reduction",
	"Ring contraction", // Python only
	"Sulfation",
	"Other",
}

var operonEvidenceMethods = vocabulary{
	"Sequence-based prediction",
	"RACE",
	"ChIPseq",
	"RNAseq",
	"rt-PCR",
}

var releaseTypeNames = vocabulary{
	"Claisen condensation",
	"Hydrolysis",
	"Macrolactamization",
	"Macrolactonization",
	"None",
	"Other",
	"Reductive release",
}

var nrpsSubclasses = vocabulary{"Type I", "Type II", "Type III", "Type IV", "Type V", "Type VI"}

var pksSubclasses = vocabulary{
	"Type I",
	"Type II aromatic",
	"Type II highly reducing",
	"Type II arylpolyene",
	"Type III",
}

var otherSubclasses = vocabulary{
	"aminocoumarin",
	"butyrolactone", // Python only, from here on
	"cyclitol",
	"ectoine",
	"fatty acid",
	"flavin",
	"indole",
	"non-nrp beta-lactam",
	"non-nrp siderophore",
	"nucleoside",
	"other",
	"pbde",
	"phenazine",
	"phosphonate",
	"shikimate-derived",
	"trna-derived",
}

var ribosomalSubclasses = vocabulary{"RiPP", "unmodified"}

var rippTypes = vocabulary{
	"Atropopeptide",
	"Biarylitide",
	"Bottromycin",
	"Borosin",
	"Crocagin",
	"Cyanobactin",
	"Cyptide",
	"Dikaritin",
	"Epipeptide",
	"Glycocin",
	"Graspetide",
	"Guanidinotide",
	"Head-to-tail cyclized peptide",
	"Lanthipeptide",
	"LAP",
	"Lasso peptide",
	"Linaridin",
	"Methanobactin",
	"Microcin",
	"Microviridin",
	"Mycofactocin",
	"Pearlin",
	"Proteusin",
	"Ranthipeptide",
	"Rotapeptide",
	"Ryptide",
	"Sactipeptide",
	"Spliceotide",
	"Streptide",
	"Sulfatyrotide",
	"Thioamidide",
	"Thiopeptide",
	"Other", // schema spelling
	"other", // Python spelling
}

var crosslinkTypes = vocabulary{"ether", "thioether", "other"}

var glycosyltransferaseEvidenceMethods = vocabulary{
	"Sequence-based prediction",
	"Structure-based inference",
	"Knock-out construct",
	"Activity assay",
}

var terpeneSubclasses = vocabulary{
	"Diterpene",
	"Hemiterpene",
	"Monoterpene",
	"Sesquiterpene",
	"Sesterterpene", // Python only
	"Triterpene",
}

var terpenePrecursors = vocabulary{"DMAPP", "FPP", "GGPP", "GPP", "IPP"}

var nonCanonicalEvidenceMethods = vocabulary{
	"Sequence-based prediction",
	"Structure-based inference",
	"Activity assay",
}

var acyltransferaseSubtypes = vocabulary{"cis-AT", "trans-AT"}

var acyltransferaseSubstrates = vocabulary{
	"acetyl-CoA", // Python only
	"malonyl-CoA",
	"methylmalonyl-CoA",
	"ethylmalonyl-CoA",
	"methoxymalonyl-CoA", // Python only
	"other",
}

var carrierSubtypes = vocabulary{"ACP", "PCP"}

var condensationSubtypes = vocabulary{
	"Dual",
	"Starter",
	"LCL",
	"DCL",
	"Ester bond-forming",
	"Heterocyclization",
}

var ketoreductaseStereochemistry = vocabulary{
	"A", // Python only
	"B", // Python only
	"A1",
	"A2",
	"B1",
	"B2",
	"C1",
	"C2",
}

var methyltransferaseSubtypes = vocabulary{"C", "N", "O", "other"}

var thioesteraseSubtypes = vocabulary{"Type I", "Type II"}

// proteinogenicAminoAcids are the names allowed for proteinogenic adenylation
// domain substrates (Python implementation).
var proteinogenicAminoAcids = vocabulary{
	"alanine",
	"arginine",
	"asparagine",
	"aspartic acid",
	"cysteine",
	"glutamine",
	"glutamic acid",
	"glycine",
	"histidine",
	"isoleucine",
	"leucine",
	"lysine",
	"methionine",
	"phenylalanine",
	"proline",
	"serine",
	"threonine",
	"tryptophan",
	"tyrosine",
	"valine",
}

// aminoAcids are the one letter codes allowed in translations.
const aminoAcids = "ACDEFGHIKLMNPQRSTVWY"
