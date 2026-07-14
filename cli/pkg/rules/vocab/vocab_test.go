package vocab

import (
	"slices"
	"testing"

	"github.com/RomanosTrechlis/3d-linter/cli/pkg/scan"
)

func TestVariants(t *testing.T) {
	for _, c := range []struct{ key, want string }{
		{"user", "users"},
		{"users", "user"}, // plural avoid word still matches singular token
		{"category", "categories"},
		{"categories", "category"},
		{"box", "boxes"},
	} {
		if !slices.Contains(variants(c.key), c.want) {
			t.Errorf("variants(%q) misses %q: %v", c.key, c.want, variants(c.key))
		}
	}
	if slices.Contains(variants("address"), "addres") {
		t.Error(`"ss" words must not be singularized`)
	}
}

func tok(key string, line int) scan.Token {
	return scan.Token{Text: key, Key: key, Line: line, Col: 1}
}

func TestNearestAncestorOwnership(t *testing.T) {
	root := &Glossary{Terms: []Term{{Use: "practitioner", Avoid: []string{"user"}}}}
	sub := &Glossary{Dir: "sub", Terms: []Term{{Use: "practitioner", Avoid: []string{"client"}}}}
	p := New([]*Glossary{sub, root}) // Discover returns deepest first
	m := &scan.Model{Files: []scan.File{
		{Path: "a.go", Tokens: []scan.Token{tok("user", 1)}},
		{Path: "sub/b.go", Tokens: []scan.Token{tok("user", 1), tok("client", 2)}},
	}}
	findings, suppressed := p.Check(m)
	if len(suppressed) != 0 || len(findings) != 2 {
		t.Fatalf("got %d findings %d suppressed, want 2/0: %+v", len(findings), len(suppressed), findings)
	}
	// sub/b.go is owned by the sub glossary: its "user" is NOT flagged, "client" is.
	if findings[0].File != "a.go" || findings[0].Match != "user" {
		t.Errorf("findings[0] = %+v", findings[0])
	}
	if findings[1].File != "sub/b.go" || findings[1].Match != "client" {
		t.Errorf("findings[1] = %+v", findings[1])
	}
}

func TestScopes(t *testing.T) {
	g := &Glossary{
		Scope: Scope{Exclude: []string{"gen/**"}},
		Terms: []Term{
			{Use: "practitioner", Avoid: []string{"user"}},
			{Use: "journal", Avoid: []string{"diary"}, Scope: &Scope{Include: []string{"**/*.md"}}},
		},
	}
	p := New([]*Glossary{g})
	m := &scan.Model{Files: []scan.File{
		{Path: "gen/x.go", Tokens: []scan.Token{tok("user", 1)}},                   // glossary-scope excluded
		{Path: "a.go", Tokens: []scan.Token{tok("diary", 1)}},                      // term-scope: not *.md
		{Path: "docs/a.md", Tokens: []scan.Token{tok("diary", 1), tok("user", 2)}}, // both hit
	}}
	findings, _ := p.Check(m)
	if len(findings) != 2 {
		t.Fatalf("got %+v, want diary+user in docs/a.md only", findings)
	}
	for _, f := range findings {
		if f.File != "docs/a.md" {
			t.Errorf("unexpected finding in %s", f.File)
		}
	}
}

func TestSuppressionHandling(t *testing.T) {
	g := &Glossary{Terms: []Term{{Use: "practitioner", Avoid: []string{"user"}}}}
	p := New([]*Glossary{g})
	m := &scan.Model{Files: []scan.File{{
		Path:   "a.go",
		Tokens: []scan.Token{tok("user", 1), tok("users", 2)},
		Allows: []scan.Allow{
			{Word: "user", Reason: "passport field", Line: 1},
			{Word: "user", Line: 2}, // reasonless — must not suppress
		},
	}}}
	findings, suppressed := p.Check(m)
	if len(suppressed) != 1 || suppressed[0].Justification != "passport field" {
		t.Fatalf("suppressed = %+v", suppressed)
	}
	if len(findings) != 1 || findings[0].Note == "" {
		t.Fatalf("reasonless suppression must yield a finding with a note, got %+v", findings)
	}
}
