package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RomanosTrechlis/3d-linter/cli/pkg/report"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGolden locks byte-identical Findings for the fixture tree (N1:
// same repo state + same glossary → identical output, on any OS).
func TestGolden(t *testing.T) {
	res, err := run(filepath.Join("testdata", "fixture"), runOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for format, ext := range map[string]string{"text": "txt", "json": "json"} {
		var buf bytes.Buffer
		if err := report.Render(&buf, format, res); err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("testdata", "golden."+ext)
		if *update {
			if err := os.WriteFile(golden, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if got, w := buf.String(), normalize(string(want)); got != w {
			t.Errorf("%s output diverged from golden:\n--- got ---\n%s--- want ---\n%s", format, got, w)
		}
	}
}

// normalize guards against CRLF checkouts mangling the comparison.
func normalize(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
