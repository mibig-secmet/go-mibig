package mibig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMarshalMatchesPython checks Marshal against the output of Python's
// json.dumps(entry, indent=2) for an entry with awkward characters: control
// characters, HTML special characters, non-ASCII characters inside and
// outside the basic multilingual plane, and U+2028.
func TestMarshalMatchesPython(t *testing.T) {
	input, err := os.ReadFile("testdata/format/input.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/format/python_output.json")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal output differs from Python's:\n%s", lineDiff(want, got))
	}
}

// TestMarshalFixtures checks that Marshal reproduces real entries as Python
// wrote them, apart from key order.
func TestMarshalFixtures(t *testing.T) {
	files, _ := filepath.Glob("testdata/entries/*.json")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := Parse(data)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		encoded, err := Marshal(entry)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if diff := jsonDiff(data, encoded); diff != "" {
			t.Errorf("%s: round trip changed the document:\n%s", file, diff)
		}
		var reindented bytes.Buffer
		if err := json.Indent(&reindented, encoded, "", "  "); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(escapeNonASCII(reindented.Bytes()), encoded) {
			t.Errorf("%s: output is not stable under re-indentation", file)
		}
	}
}

func TestEscapeNonASCII(t *testing.T) {
	tests := map[string]string{
		`"plain"`:  `"plain"`,
		`"é"`:      `"\u00e9"`,
		`"🧬"`:      `"\ud83e\uddec"`,
		"\"\x7f\"": `"\u007f"`,
	}
	for input, want := range tests {
		if got := string(escapeNonASCII([]byte(input))); got != want {
			t.Errorf("escapeNonASCII(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestWriteFile(t *testing.T) {
	entry, err := ReadFile("testdata/entries/BGC0002938.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "entry.json")

	if err := WriteFile(name, entry); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("new file has mode %v, want 0644", info.Mode().Perm())
	}

	// Existing files keep their permissions.
	if err := os.Chmod(name, 0o600); err != nil {
		t.Fatal(err)
	}
	entry.Comment = "updated"
	if err := WriteFile(name, entry); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Stat(name)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("rewritten file has mode %v, want 0600", info.Mode().Perm())
	}

	reread, err := ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if reread.Comment != "updated" {
		t.Errorf("comment not updated")
	}

	leftovers, _ := filepath.Glob(filepath.Join(dir, ".*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

// lineDiff shows the first differing line of two texts.
func lineDiff(want, got []byte) string {
	wantLines := bytes.Split(want, []byte("\n"))
	gotLines := bytes.Split(got, []byte("\n"))
	for i := range max(len(wantLines), len(gotLines)) {
		var w, g []byte
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if !bytes.Equal(w, g) {
			return fmt.Sprintf("line %d:\n  want: %s\n  got:  %s", i+1, w, g)
		}
	}
	return ""
}
