package mibig_test

import (
	"encoding/json"
	"fmt"
	"log"

	mibig "go.kblin.dev/go-mibig"
)

func ExampleReadFile() {
	entry, err := mibig.ReadFile("testdata/entries/BGC0002938.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(entry.AccessionVersion(), entry.Status, entry.Taxonomy.Name)
	// Output: BGC0002938.1 retired Fusarium graminearum F15
}

func ExampleEntry_Validate() {
	entry, err := mibig.ReadFile("testdata/entries/BGC0000553.json")
	if err != nil {
		log.Fatal(err)
	}
	issues := entry.Validate()
	for _, issue := range issues {
		fmt.Println(issue)
	}
	fmt.Println("valid:", issues.Err() == nil)
	// Output:
	// biosynthesis.classes[0].precursors[0].core_sequence: warning: core sequence is stored as a list, use a single string [legacy-format]
	// biosynthesis.classes[0].precursors[0].crosslinks[0].type: error: "Thioether" is not a valid crosslink type [invalid-value]
	// biosynthesis.classes[0].precursors[0].crosslinks[1].type: error: "Thioether" is not a valid crosslink type [invalid-value]
	// valid: false
}

func ExampleEntry_ValidateAt() {
	entry, err := mibig.ReadFile("testdata/entries/BGC0001164.json")
	if err != nil {
		log.Fatal(err)
	}
	// Check what is missing before the entry can be promoted to high quality.
	fmt.Println("quality:", entry.Quality)
	fmt.Println("errors at quality high:", len(entry.ValidateAt(mibig.QualityHigh).Errors()))
	// Output:
	// quality: questionable
	// errors at quality high: 13
}

func ExampleModules() {
	entry, err := mibig.ReadFile("testdata/entries/BGC0000370.json")
	if err != nil {
		log.Fatal(err)
	}
	for _, module := range entry.Biosynthesis.Modules {
		switch m := module.(type) {
		case *mibig.NRPSType1Module:
			if m.ADomain == nil {
				continue
			}
			for _, substrate := range m.ADomain.Substrates {
				fmt.Printf("module %s: %s\n", m.Name, substrate.Name)
			}
		default:
			fmt.Printf("module %s: %s\n", module.Base().Name, module.ModuleType())
		}
	}
	// Output:
	// module 1: proline
	// module 2: alanine
	// module 3: D-alanine
}

func ExampleMarshal() {
	entry := &mibig.Entry{
		Accession:    "BGC0099999",
		Version:      1,
		Quality:      mibig.QualityQuestionable,
		Status:       mibig.StatusPending,
		Completeness: mibig.CompletenessUnknown,
		Loci: []mibig.Locus{{
			Accession: "AB000001.1",
			Location:  mibig.Location{From: 1, To: 20000},
		}},
		Biosynthesis: mibig.Biosynthesis{
			Classes: mibig.Classes{&mibig.TerpeneClass{Subclass: "Diterpene", Precursor: "GGPP"}},
		},
		Compounds: []mibig.Compound{{Name: "exampline α"}},
		Taxonomy:  mibig.Taxonomy{Name: "Streptomyces sp.", NCBITaxID: 1931},
	}
	data, err := mibig.Marshal(entry)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
	// Output:
	// {
	//   "accession": "BGC0099999",
	//   "version": 1,
	//   "changelog": {
	//     "releases": []
	//   },
	//   "quality": "questionable",
	//   "status": "pending",
	//   "completeness": "unknown",
	//   "loci": [
	//     {
	//       "accession": "AB000001.1",
	//       "location": {
	//         "from": 1,
	//         "to": 20000
	//       },
	//       "evidence": []
	//     }
	//   ],
	//   "biosynthesis": {
	//     "classes": [
	//       {
	//         "class": "terpene",
	//         "subclass": "Diterpene",
	//         "precursor": "GGPP"
	//       }
	//     ]
	//   },
	//   "compounds": [
	//     {
	//       "name": "exampline \u03b1",
	//       "evidence": []
	//     }
	//   ],
	//   "taxonomy": {
	//     "name": "Streptomyces sp.",
	//     "ncbiTaxId": 1931
	//   }
	// }
}

// Migrating the legacy "synthases" key of terpene classes to
// "synthases_cyclases".
func ExampleTerpeneClass_migrate() {
	entry, err := mibig.ReadFile("testdata/entries/BGC0002938.json")
	if err != nil {
		log.Fatal(err)
	}
	for _, class := range entry.Biosynthesis.Classes {
		if terpene, ok := class.(*mibig.TerpeneClass); ok && len(terpene.Synthases) > 0 {
			terpene.SynthasesCyclases = append(terpene.SynthasesCyclases, terpene.Synthases...)
			terpene.Synthases = nil
		}
	}
	for _, issue := range entry.Validate() {
		fmt.Println(issue)
	}
	// Output:
	// compounds: error: at least one compound is required [required]
}

// Decoding preserves empty values the Go fields cannot express, such as
// "comments": null, for as long as the field holds its zero value. Discard
// drops the preserved form, so that clearing the field deletes the key.
func ExampleExtras_Discard() {
	var module mibig.OtherModule
	input := `{"type": "other", "name": "m1", "genes": ["a"], "active": true, "comments": null}`
	if err := json.Unmarshal([]byte(input), &module); err != nil {
		log.Fatal(err)
	}
	show := func() {
		encoded, err := json.Marshal(module)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(encoded))
	}

	module.Comment = "checked"
	show()
	// Clearing the field brings back the input form.
	module.Comment = ""
	show()
	// Discarding it deletes the key.
	module.Extra.Discard("comment")
	show()
	// Output:
	// {"type":"other","name":"m1","genes":["a"],"active":true,"comment":"checked"}
	// {"type":"other","name":"m1","genes":["a"],"active":true,"comments":null}
	// {"type":"other","name":"m1","genes":["a"],"active":true}
}
