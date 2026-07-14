// Package vocab is Rule Pack #1 (spec §6): glossary load, avoid-list
// matching with trivial plural/casing variants, scopes, suppressions.
package vocab

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/RomanosTrechlis/3d-linter/cli/pkg/engine"
	"github.com/RomanosTrechlis/3d-linter/cli/pkg/scan"
)

// Scope is a set of include/exclude globs (doublestar syntax, e.g. "docs/**"),
// relative to the directory holding the glossary.
type Scope struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// Match reports whether rel (slash path relative to the glossary dir) is in scope.
func (s *Scope) Match(rel string) bool {
	if s == nil {
		return true
	}
	if len(s.Include) > 0 && !anyMatch(s.Include, rel) {
		return false
	}
	return !anyMatch(s.Exclude, rel)
}

func anyMatch(patterns []string, rel string) bool {
	for _, p := range patterns {
		if ok, _ := doublestar.Match(p, rel); ok {
			return true
		}
	}
	return false
}

// Term is one canonical concept (spec §3).
type Term struct {
	Use        string   `yaml:"use"`
	Avoid      []string `yaml:"avoid"`
	Definition string   `yaml:"definition"`
	Reason     string   `yaml:"reason"`
	Scope      *Scope   `yaml:"scope"`
}

// Glossary is the versioned YAML file defining Terms — the single source of
// truth (F1). A repo may hold several; the nearest ancestor owns a file (F2).
type Glossary struct {
	Scope Scope  `yaml:"scope"`
	Terms []Term `yaml:"terms"`

	Dir  string `yaml:"-"` // slash dir relative to scan root ("" = root)
	Path string `yaml:"-"` // for error messages
}

// Load reads one glossary file. dir is its directory relative to the scan root.
func Load(path, dir string) (*Glossary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g := &Glossary{Dir: dir, Path: path}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(g); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	for i, t := range g.Terms {
		if strings.TrimSpace(t.Use) == "" {
			return nil, fmt.Errorf("%s: terms[%d] is missing `use`", path, i)
		}
	}
	return g, nil
}

// Discover finds every .glossary.yml under root, skipping the same
// directories the scanner skips.
func Discover(root string) ([]*Glossary, error) {
	var gs []*Glossary
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil // unreadable entries are not repo content
		}
		if d.IsDir() {
			if path != root && scan.SkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !scan.IsGlossary(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		dir := filepath.ToSlash(rel)
		if dir == "." {
			dir = ""
		}
		g, err := Load(path, dir)
		if err != nil {
			return err
		}
		gs = append(gs, g)
		return nil
	})
	// Deepest first, so the nearest ancestor wins when assigning files.
	sort.SliceStable(gs, func(i, j int) bool { return len(gs[i].Dir) > len(gs[j].Dir) })
	return gs, err
}

type hit struct {
	term  *Term
	avoid string
}

// Pack implements engine.Pack for the vocabulary rules.
type Pack struct {
	glossaries []*Glossary
	matchers   []map[string]hit  // parallel to glossaries
	useKeys    []map[string]bool // parallel: normalized canonical `use` terms
}

// New builds the pack, pre-compiling one variant→term index per glossary.
func New(gs []*Glossary) *Pack {
	p := &Pack{glossaries: gs}
	for _, g := range gs {
		m := map[string]hit{}
		uk := map[string]bool{}
		for i := range g.Terms {
			t := &g.Terms[i]
			uk[scan.Key(t.Use)] = true
			for _, w := range t.Avoid {
				for _, v := range variants(scan.Key(w)) {
					if _, dup := m[v]; !dup {
						m[v] = hit{term: t, avoid: w}
					}
				}
			}
		}
		p.matchers = append(p.matchers, m)
		p.useKeys = append(p.useKeys, uk)
	}
	return p
}

// Name implements engine.Pack.
func (p *Pack) Name() string { return "vocab" }

// Check implements engine.Pack.
func (p *Pack) Check(m *scan.Model) (findings, suppressed []engine.Finding) {
	for _, f := range m.Files {
		gi := p.owner(f.Path)
		if gi < 0 {
			continue
		}
		g, matcher := p.glossaries[gi], p.matchers[gi]
		rel := relTo(g.Dir, f.Path)
		if !g.Scope.Match(rel) {
			continue
		}
		for i, tok := range f.Tokens {
			h, ok := matcher[tok.Key]
			if !ok || !h.term.Scope.Match(rel) {
				continue
			}
			if p.partOfUseTerm(gi, f.Tokens, i) {
				continue
			}
			fd := engine.Finding{
				File: f.Path, Line: tok.Line, Col: tok.Col,
				Match: tok.Text, Use: h.term.Use, Reason: h.term.Reason,
				Rule: p.Name(),
			}
			if fd.Reason == "" {
				fd.Reason = h.term.Definition
			}
			switch allow := allowFor(f.Allows, tok); {
			case allow != nil && allow.Reason != "":
				fd.Justification = allow.Reason
				suppressed = append(suppressed, fd)
			case allow != nil:
				fd.Note = "suppression ignored — a reason is required: 3dl:allow " + allow.Word + " -- why"
				findings = append(findings, fd)
			default:
				findings = append(findings, fd)
			}
		}
	}
	return findings, suppressed
}

// owner returns the index of the nearest-ancestor glossary for path (F2), or
// -1. The nearest ancestor owns the file outright — its scope decides
// inclusion; there is no fall-through to a higher glossary.
func (p *Pack) owner(path string) int {
	for i, g := range p.glossaries {
		if g.Dir == "" || strings.HasPrefix(path, g.Dir+"/") {
			return i
		}
	}
	return -1
}

func relTo(dir, path string) string {
	if dir == "" {
		return path
	}
	return strings.TrimPrefix(path, dir+"/")
}

// partOfUseTerm reports whether the token sits inside one of this glossary's
// canonical `use` terms, so canonical usage is never flagged: with
// `use: vice mark` + `avoid: [mark]`, both `viceMark` and prose "vice mark"
// pass while a bare "mark" still fails.
// ponytail: exact identifier + adjacent word pairs only; 3+-word prose terms
// wait for real demand.
func (p *Pack) partOfUseTerm(gi int, toks []scan.Token, i int) bool {
	uk := p.useKeys[gi]
	tok := toks[i]
	// The whole identifier is a canonical term (addViceMark handled below).
	if tok.Parent != "" && uk[tok.Parent] {
		return true
	}
	// Pairs: an adjacent sibling word — same identifier (gap 0–1 across a
	// case/-/_ boundary) or same prose line (gap 1, the space).
	for _, j := range []int{i - 1, i + 1} {
		if j < 0 || j >= len(toks) {
			continue
		}
		n := toks[j]
		if n.Line != tok.Line || n.Parent != tok.Parent {
			continue
		}
		first, second := n, tok
		if j > i {
			first, second = tok, n
		}
		gap := second.Col - (first.Col + len(first.Text))
		if gap < 0 || gap > 1 {
			continue
		}
		for _, v := range variants(first.Key + second.Key) {
			if uk[v] {
				return true
			}
		}
	}
	return false
}

// allowFor returns the first suppression on the token's line that names it.
func allowFor(allows []scan.Allow, tok scan.Token) *scan.Allow {
	for i := range allows {
		a := &allows[i]
		if a.Line != tok.Line {
			continue
		}
		if slices.Contains(variants(scan.Key(a.Word)), tok.Key) {
			return a
		}
	}
	return nil
}

// variants expands a normalized key to its trivial plural/singular forms
// (F5: `user`/`users` — nothing fancier, precision over recall).
// ponytail: s/es/ies only; stemming is explicitly out of scope forever.
func variants(k string) []string {
	set := []string{k, k + "s"}
	if sibilant(k) {
		set = append(set, k+"es") // box → boxes, but never tim → times
	}
	switch {
	case strings.HasSuffix(k, "ies"):
		set = append(set, k[:len(k)-3]+"y")
	case strings.HasSuffix(k, "y"):
		set = append(set, k[:len(k)-1]+"ies")
	}
	if strings.HasSuffix(k, "es") && sibilant(k[:len(k)-2]) {
		set = append(set, k[:len(k)-2])
	}
	if strings.HasSuffix(k, "s") && !strings.HasSuffix(k, "ss") {
		set = append(set, k[:len(k)-1])
	}
	return set
}

// sibilant reports whether a word forms its plural with "es".
func sibilant(k string) bool {
	return strings.HasSuffix(k, "s") || strings.HasSuffix(k, "x") || strings.HasSuffix(k, "z") ||
		strings.HasSuffix(k, "ch") || strings.HasSuffix(k, "sh")
}
