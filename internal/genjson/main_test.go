package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratedFileIsUpToDate fails if json_gen.go needs to be regenerated
// with go generate.
func TestGeneratedFileIsUpToDate(t *testing.T) {
	dir := filepath.Join("..", "..")
	want, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, output))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date, run go generate", output)
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	src := `package p

type Extras struct{}

type Plain struct {
	A     int
	Extra Extras
}

type Custom struct{ Extra Extras }

func (c Custom) MarshalJSON() ([]byte, error) { return nil, nil }

type Decoder struct{ Extra Extras }

func (d *Decoder) UnmarshalJSON(data []byte) error { return nil }

type Both struct{ Extra Extras }

func (b Both) MarshalJSON() ([]byte, error)     { return nil, nil }
func (b *Both) UnmarshalJSON(data []byte) error { return nil }

type NoExtras struct{ A int }
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func (x *Custom) UnmarshalJSON(",
		"func (x Decoder) MarshalJSON(",
		"func (x Plain) MarshalJSON(",
		"func (x *Plain) UnmarshalJSON(",
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Custom) MarshalJSON", "Decoder) UnmarshalJSON", "Both)", "NoExtras"} {
		if bytes.Contains(got, []byte(unwanted)) {
			t.Errorf("unexpected %q in\n%s", unwanted, got)
		}
	}
}
