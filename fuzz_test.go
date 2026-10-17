package mibig

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

// FuzzParse checks that Parse does not panic, and that every entry it
// accepts can be validated, round-trips to the same JSON document and
// encodes stably.
func FuzzParse(f *testing.F) {
	files, _ := filepath.Glob("testdata/entries/*.json")
	files = append(files, "testdata/schema-complete.json")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(`{"biosynthesis":{"classes":[{"class":"x"},null],"modules":[{"type":"cal","cal":{"type":"y"}}]}}`))
	f.Add([]byte(`{"compounds":[{"name":{"activity":1}}],"genes":{"annotations":[{"domains":[{"type":"ligase","substrate":"C"}]}]}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		entry, err := Parse(data)
		if err != nil {
			return
		}
		_ = entry.Validate()
		_ = entry.References()

		encoded, err := Marshal(entry)
		if err != nil {
			t.Fatalf("Marshal failed for a parsed entry: %v", err)
		}
		// Invalid UTF-8 is replaced when decoding, so it cannot round-trip.
		if utf8.Valid(data) {
			if diff := jsonDiffNormalised(data, encoded); diff != "" {
				t.Fatalf("round trip changed the document:\n%s\ninput: %s\noutput: %s", diff, data, encoded)
			}
		}
		again, err := Parse(encoded)
		if err != nil {
			t.Fatalf("cannot parse own output: %v\n%s", err, encoded)
		}
		encodedAgain, err := Marshal(again)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, encodedAgain) {
			t.Fatalf("encoding is not stable:\n%s\n%s", encoded, encodedAgain)
		}
	})
}
