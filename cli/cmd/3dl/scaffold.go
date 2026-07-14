package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// N5 — no ceremony: `3dl init` scaffolds a starter glossary and Action
// workflow in one command.

const glossaryTemplate = `# .glossary.yml — your team's domain language, enforced by 3dl.
# Every term: a canonical name (use), forbidden synonyms (avoid),
# and ideally a reason — the reason is what shows up in PR annotations.
scope:
  exclude:
    - "**/generated/**"
terms:
  - use: practitioner # example — replace with your own domain terms
    avoid: [user, customer]
    definition: A person following their daily practice.
    reason: '"user" is too generic and "customer" implies payment'
`

const workflowTemplate = `name: vocabulary gate
on:
  pull_request:
jobs:
  3dl:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # full history so diff-only mode can find the merge base
      # consider pinning to a release tag or commit SHA
      - uses: RomanosTrechlis/3d-linter/github-action@master
`

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Scaffold a starter .glossary.yml and GitHub Actions workflow",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := pathArg(args)
			created := 0
			for _, f := range []struct{ path, content string }{
				{filepath.Join(dir, ".glossary.yml"), glossaryTemplate},
				{filepath.Join(dir, ".github", "workflows", "3dl.yml"), workflowTemplate},
			} {
				ok, err := writeIfAbsent(f.path, f.content)
				if err != nil {
					return err
				}
				if ok {
					fmt.Println("created", f.path)
					created++
				} else {
					fmt.Println("exists, skipped", f.path)
				}
			}
			if created > 0 {
				fmt.Println("\nNext: put your real terms in .glossary.yml, then open a PR — the gate does the rest.")
			}
			return nil
		},
	}
}

func writeIfAbsent(path, content string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(content), 0o644)
}
