// 3dl — the deterministic domain-vocabulary gate (F10).
// Single static binary, no network calls, exit 0/1 (2 on errors).
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/RomanosTrechlis/3d-linter/cli/pkg/diff"
	"github.com/RomanosTrechlis/3d-linter/cli/pkg/engine"
	"github.com/RomanosTrechlis/3d-linter/cli/pkg/report"
	"github.com/RomanosTrechlis/3d-linter/cli/pkg/rules/vocab"
	"github.com/RomanosTrechlis/3d-linter/cli/pkg/scan"
)

var version = "0.1.0-dev" // set via -ldflags "-X main.version=..."

// errFindings signals exit code 1 without an error message (the findings are
// the message).
var errFindings = errors.New("findings")

func main() {
	root := &cobra.Command{
		Use:           "3dl",
		Short:         "Enforce your domain glossary in code, comments, and docs — deterministically.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(checkCmd(), auditCmd(), initCmd())
	if err := root.Execute(); err != nil {
		if errors.Is(err, errFindings) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "3dl:", err)
		os.Exit(2)
	}
}

type runOpts struct {
	glossary string // explicit glossary path (F1 "path configurable"); empty = discover
	base     string // diff base ref; empty = full scan
	strings  bool   // scan string literals (F4 opt-in)
}

// run wires scan → packs → engine for a tree. Shared by check, audit, and the
// golden test (N1).
func run(dir string, o runOpts) (engine.Result, error) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return engine.Result{}, err
	}
	var gs []*vocab.Glossary
	if o.glossary != "" {
		g, err := vocab.Load(o.glossary, "")
		if err != nil {
			return engine.Result{}, err
		}
		gs = []*vocab.Glossary{g}
	} else if gs, err = vocab.Discover(abs); err != nil {
		return engine.Result{}, err
	}
	if len(gs) == 0 {
		return engine.Result{}, fmt.Errorf("no .glossary.yml found under %s — run `3dl init`", dir)
	}
	model, err := scan.Walk(abs, scan.Options{Strings: o.strings})
	if err != nil {
		return engine.Result{}, err
	}
	var changed diff.LineSet
	if o.base != "" {
		if changed, err = diff.Changed(abs, o.base); err != nil {
			return engine.Result{}, err
		}
	}
	return engine.Run(model, changed, []engine.Pack{vocab.New(gs)}), nil
}

func checkCmd() *cobra.Command {
	var o runOpts
	var format string
	c := &cobra.Command{
		Use:   "check [path]",
		Short: "Gate: scan and exit 1 if Findings exist (use --diff for diff-only mode)",
		Long: "Scans the tree and exits 1 when Findings exist.\n" +
			"With --diff BASE, only lines added or modified since BASE are gated\n" +
			"(diff-only mode — the default posture for CI).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := run(pathArg(args), o)
			if err != nil {
				return err
			}
			if err := report.Render(os.Stdout, format, res); err != nil {
				return err
			}
			if len(res.Findings) > 0 {
				return errFindings
			}
			return nil
		},
	}
	addCommonFlags(c, &o, &format)
	c.Flags().StringVar(&o.base, "diff", "", "git base ref: gate only lines added/modified since this ref")
	return c
}

func auditCmd() *cobra.Command {
	var o runOpts
	var format string
	var gate bool
	c := &cobra.Command{
		Use:   "audit [path]",
		Short: "Full-repo scan with an advisory exit code (0 even with Findings)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := run(pathArg(args), o)
			if err != nil {
				return err
			}
			if err := report.Render(os.Stdout, format, res); err != nil {
				return err
			}
			if gate && len(res.Findings) > 0 {
				return errFindings
			}
			return nil
		},
	}
	addCommonFlags(c, &o, &format)
	c.Flags().BoolVar(&gate, "gate", false, "exit 1 when Findings exist")
	return c
}

func addCommonFlags(c *cobra.Command, o *runOpts, format *string) {
	c.Flags().StringVar(&o.glossary, "glossary", "", "use one specific glossary file instead of discovery")
	c.Flags().BoolVar(&o.strings, "strings", false, "also scan string literal contents (off by default)")
	c.Flags().StringVar(format, "format", "text", "output format: text|json|sarif|github")
}

func pathArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}
