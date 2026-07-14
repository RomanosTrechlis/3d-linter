// Package scan walks a source tree and extracts word-level tokens from
// identifiers and prose (F4). Language-agnostic regex extraction — tree-sitter
// declaration awareness (F8) is the v1 upgrade behind this same model.
package scan

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Model is the parsed source tree every Rule Pack consumes (spec §6).
type Model struct {
	Root  string
	Files []File
}

// File is one scanned file with its tokens and inline suppressions.
type File struct {
	Path   string // slash-separated, relative to Root
	Tokens []Token
	Allows []Allow
}

// Token is one matchable unit: a sub-word of a split identifier
// ("getUserName" → get, User, Name) or a whole separator-joined identifier
// with separators collapsed ("black_list" → key "blacklist") so compound
// avoid terms match too.
type Token struct {
	Text   string // as written in the source
	Key    string // lowercased, separators removed — what matching runs on
	Line   int    // 1-based
	Col    int    // 1-based byte column
	Parent string // for identifier sub-words: the whole identifier's key ("" otherwise)
}

// Allow is one inline suppression: `3dl:allow word -- reason` (F7).
// Reason is mandatory; an Allow with an empty Reason never suppresses.
type Allow struct {
	Word   string
	Reason string
	Line   int
}

// Options controls extraction.
type Options struct {
	Strings bool // scan string literal contents too (opt-in per F4)
}

var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "testdata": true,
	"dist": true, "build": true, "target": true, "__pycache__": true,
}

// SkipDir reports whether a directory name is never scanned (also used by
// glossary discovery so both walks agree).
func SkipDir(name string) bool { return skipDirs[name] }

var skipExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".pdf": true, ".zip": true, ".gz": true, ".tar": true, ".jar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".svg": true, ".map": true, ".lock": true,
}

var skipNames = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "composer.lock": true, "Cargo.lock": true,
}

// prose files are scanned whole; everything else gets string literals masked
// unless Options.Strings is set.
var proseExts = map[string]bool{
	".md": true, ".markdown": true, ".mdx": true, ".txt": true,
	".rst": true, ".adoc": true,
}

const maxFileSize = 1 << 20 // 1 MiB — bigger files are generated, not written

// IsGlossary reports whether name is a glossary file. Glossary files are
// never scanned: they list the avoided words by definition.
func IsGlossary(name string) bool {
	return name == ".glossary.yml" || name == ".glossary.yaml"
}

// Walk scans the tree rooted at root and returns the Model.
func Walk(root string, opt Options) (*Model, error) {
	m := &Model{Root: root}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && SkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if skipNames[name] || skipExts[strings.ToLower(filepath.Ext(name))] ||
			IsGlossary(name) || strings.Contains(name, ".min.") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			return nil // binary
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		f := scanFile(filepath.ToSlash(rel), data, opt)
		if len(f.Tokens) > 0 || len(f.Allows) > 0 {
			m.Files = append(m.Files, f)
		}
		return nil
	})
	return m, err
}

var (
	rawTokenRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:[-_][A-Za-z0-9]+)*`)
	allowRe    = regexp.MustCompile(`(?i)3dl:allow\s+([A-Za-z0-9_-]+)(?:\s+--\s*(.+))?`)
)

func scanFile(rel string, data []byte, opt Options) File {
	f := File{Path: rel}
	prose := proseExts[strings.ToLower(filepath.Ext(rel))]
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		ln := i + 1

		// Suppression directives are recorded, then masked so their own text
		// (the allowed word, the reason) is not tokenized.
		for _, m := range allowRe.FindAllStringSubmatchIndex(line, -1) {
			a := Allow{Word: line[m[2]:m[3]], Line: ln}
			if m[4] >= 0 {
				a.Reason = cleanReason(line[m[4]:m[5]])
			}
			f.Allows = append(f.Allows, a)
			line = line[:m[0]] + strings.Repeat(" ", m[1]-m[0]) + line[m[1]:]
		}

		if !prose && !opt.Strings {
			line = maskStrings(line)
		}
		f.Tokens = append(f.Tokens, tokenize(line, ln)...)
	}
	return f
}

// cleanReason trims comment closers so `/* 3dl:allow x -- why */` keeps "why".
func cleanReason(s string) string {
	s = strings.TrimSpace(s)
	for _, suf := range []string{"*/", "-->", "#}"} {
		s = strings.TrimSpace(strings.TrimSuffix(s, suf))
	}
	return s
}

// maskStrings blanks the contents of single-line string literals, preserving
// byte columns. A quote directly after a letter/digit is treated as an
// apostrophe (don't, user's), not an opener.
// ponytail: line-local heuristic — multi-line strings pass through; per-language
// accuracy is F8 (tree-sitter), not more regex.
func maskStrings(line string) string {
	b := []byte(line)
	var q byte
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case q == 0:
			if c == '"' || c == '`' || (c == '\'' && (i == 0 || !isWordByte(b[i-1]))) {
				q = c
			}
		case c == '\\' && q != '`':
			b[i] = ' '
			if i+1 < len(b) {
				b[i+1] = ' '
				i++
			}
		case c == q:
			q = 0
		default:
			b[i] = ' '
		}
	}
	return string(b)
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func tokenize(line string, ln int) []Token {
	var out []Token
	for _, loc := range rawTokenRe.FindAllStringIndex(line, -1) {
		raw := line[loc[0]:loc[1]]
		words := splitWords(raw)
		if len(words) == 1 {
			out = append(out, Token{Text: raw, Key: strings.ToLower(raw), Line: ln, Col: loc[0] + 1})
			continue
		}
		compound := Key(raw)
		for _, w := range words {
			out = append(out, Token{
				Text: raw[w.start:w.end], Key: strings.ToLower(raw[w.start:w.end]),
				Line: ln, Col: loc[0] + w.start + 1, Parent: compound,
			})
		}
		// The compound form catches multi-word avoid terms in identifiers:
		// avoid "blacklist" must match `black_list` and `blackList`.
		out = append(out, Token{Text: raw, Key: compound, Line: ln, Col: loc[0] + 1})
	}
	return out
}

// Key normalizes a term or token for matching: lowercase, separators removed.
func Key(s string) string {
	s = strings.ToLower(s)
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(s)
}

type span struct{ start, end int }

// splitWords splits an identifier on -/_ separators and camelCase boundaries
// (getUserName → get, User, Name; HTTPServer → HTTP, Server).
func splitWords(s string) []span {
	var out []span
	start := 0
	flush := func(end int) {
		if end > start {
			out = append(out, span{start, end})
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '_' {
			flush(i)
			start = i + 1
			continue
		}
		if i > start && isUpper(c) {
			prev := s[i-1]
			nextLower := i+1 < len(s) && isLower(s[i+1])
			if isLower(prev) || isDigit(prev) || (isUpper(prev) && nextLower) {
				flush(i)
				start = i
			}
		}
	}
	flush(len(s))
	return out
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
