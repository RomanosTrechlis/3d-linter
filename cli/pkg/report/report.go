// Package report renders engine Results — rule-pack-agnostic (spec §6).
// Formats: text, json, sarif (F10) and github workflow-command annotations
// (resolves spec open item 3 without the code-scanning upload dependency).
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/RomanosTrechlis/3d-linter/cli/pkg/engine"
)

// Formats lists the supported --format values.
var Formats = []string{"text", "json", "sarif", "github"}

// Render writes res to w in the given format.
func Render(w io.Writer, format string, res engine.Result) error {
	switch format {
	case "text":
		return text(w, res)
	case "json":
		return jsonOut(w, res)
	case "sarif":
		return sarif(w, res)
	case "github":
		return github(w, res)
	default:
		return fmt.Errorf("unknown format %q (want %s)", format, strings.Join(Formats, "|"))
	}
}

func summary(res engine.Result) string {
	s := fmt.Sprintf("%d finding(s)", len(res.Findings))
	if n := len(res.Suppressed); n > 0 {
		s += fmt.Sprintf(", %d suppressed", n)
	}
	return s
}

func text(w io.Writer, res engine.Result) error {
	for _, f := range res.Findings {
		fmt.Fprintf(w, "%s:%d:%d: %s [%s]\n", f.File, f.Line, f.Col, f.Message(), f.Rule)
	}
	if len(res.Findings) == 0 {
		fmt.Fprint(w, "No findings.")
		if n := len(res.Suppressed); n > 0 {
			fmt.Fprintf(w, " (%d suppressed)", n)
		}
		fmt.Fprintln(w)
		return nil
	}
	fmt.Fprintln(w, summary(res))
	return nil
}

func jsonOut(w io.Writer, res engine.Result) error {
	out := struct {
		engine.Result
		Counts map[string]int `json:"counts"`
	}{res, map[string]int{"findings": len(res.Findings), "suppressed": len(res.Suppressed)}}
	if out.Findings == nil {
		out.Findings = []engine.Finding{}
	}
	if out.Suppressed == nil {
		out.Suppressed = []engine.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// --- SARIF 2.1.0 (minimal) ---

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}
type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}
type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}
type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}
type sarifRule struct {
	ID        string    `json:"id"`
	ShortDesc sarifText `json:"shortDescription"`
}
type sarifText struct {
	Text string `json:"text"`
}
type sarifResult struct {
	RuleID       string             `json:"ruleId"`
	Level        string             `json:"level"`
	Message      sarifText          `json:"message"`
	Locations    []sarifLocation    `json:"locations"`
	Suppressions []sarifSuppression `json:"suppressions,omitempty"`
}
type sarifLocation struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region struct {
			StartLine   int `json:"startLine"`
			StartColumn int `json:"startColumn"`
		} `json:"region"`
	} `json:"physicalLocation"`
}
type sarifSuppression struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification,omitempty"`
}

var ruleDescriptions = map[string]string{
	"vocab": "Domain vocabulary: term is on the glossary avoid list",
}

func sarif(w io.Writer, res engine.Result) error {
	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "3dl", InformationURI: "https://github.com/RomanosTrechlis/3d-linter"}},
			Results: []sarifResult{},
		}},
	}
	seen := map[string]bool{}
	addRule := func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		desc := ruleDescriptions[id]
		if desc == "" {
			desc = id
		}
		log.Runs[0].Tool.Driver.Rules = append(log.Runs[0].Tool.Driver.Rules, sarifRule{ID: id, ShortDesc: sarifText{Text: desc}})
	}
	add := func(f engine.Finding, sup []sarifSuppression) {
		addRule(f.Rule)
		r := sarifResult{RuleID: f.Rule, Level: "error", Message: sarifText{Text: f.Message()}, Suppressions: sup}
		var loc sarifLocation
		loc.PhysicalLocation.ArtifactLocation.URI = f.File
		loc.PhysicalLocation.Region.StartLine = f.Line
		loc.PhysicalLocation.Region.StartColumn = f.Col
		r.Locations = []sarifLocation{loc}
		log.Runs[0].Results = append(log.Runs[0].Results, r)
	}
	for _, f := range res.Findings {
		add(f, nil)
	}
	for _, f := range res.Suppressed {
		add(f, []sarifSuppression{{Kind: "inSource", Justification: f.Justification}})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

// --- GitHub workflow commands ---

var propEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
var msgEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")

func github(w io.Writer, res engine.Result) error {
	for _, f := range res.Findings {
		fmt.Fprintf(w, "::error file=%s,line=%d,col=%d,title=3D-Linter (%s)::%s\n",
			propEscaper.Replace(f.File), f.Line, f.Col, propEscaper.Replace(f.Rule), msgEscaper.Replace(f.Message()))
	}
	fmt.Fprintln(w, summary(res))
	return nil
}
