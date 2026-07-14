// Package engine runs Rule Packs over a scan model and merges their Findings
// (spec §6 — the pivot). The Rule Pack abstraction is deliberately one
// interface; vocabulary is the only pack in v0.
package engine

import (
	"fmt"
	"sort"

	"github.com/RomanosTrechlis/3d-linter/cli/pkg/diff"
	"github.com/RomanosTrechlis/3d-linter/cli/pkg/scan"
)

// Finding is one flagged occurrence (spec §3): file, line, matched token,
// the canonical Term, the reason.
type Finding struct {
	File          string `json:"file"`
	Line          int    `json:"line"`
	Col           int    `json:"col"`
	Match         string `json:"match"`
	Use           string `json:"use"`
	Reason        string `json:"reason,omitempty"`
	Rule          string `json:"rule"`
	Note          string `json:"note,omitempty"`
	Justification string `json:"suppress_reason,omitempty"` // set on suppressed Findings
}

// Message renders the human-readable message for a Finding.
func (f Finding) Message() string {
	msg := fmt.Sprintf("%q is on the avoid list — use %q", f.Match, f.Use)
	if f.Reason != "" {
		msg += ": " + f.Reason
	}
	if f.Note != "" {
		msg += " [" + f.Note + "]"
	}
	return msg
}

// Pack is one family of deterministic checks (spec §3 "Rule Pack").
// Packs return active and suppressed Findings; diff filtering is applied by
// the engine so every pack gets it for free.
type Pack interface {
	Name() string
	Check(m *scan.Model) (findings, suppressed []Finding)
}

// Result is a deterministic, sorted run outcome (N1).
type Result struct {
	Findings   []Finding `json:"findings"`
	Suppressed []Finding `json:"suppressed"`
}

// Run executes all packs and, when changed is non-nil, keeps only Findings on
// added/modified lines (F6 diff-only mode).
func Run(m *scan.Model, changed diff.LineSet, packs []Pack) Result {
	var res Result
	for _, p := range packs {
		f, s := p.Check(m)
		res.Findings = append(res.Findings, filter(f, changed)...)
		res.Suppressed = append(res.Suppressed, filter(s, changed)...)
	}
	sortFindings(res.Findings)
	sortFindings(res.Suppressed)
	return res
}

func filter(fs []Finding, changed diff.LineSet) []Finding {
	if changed == nil {
		return fs
	}
	var out []Finding
	for _, f := range fs {
		if changed[f.File][f.Line] {
			out = append(out, f)
		}
	}
	return out
}

func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.Match != b.Match {
			return a.Match < b.Match
		}
		return a.Use < b.Use
	})
}
