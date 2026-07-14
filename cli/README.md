# 3dl — the CLI

Single static Go binary. No network calls, no telemetry, no accounts (N4). Exit codes: `0` clean, `1` Findings exist, `2` error.

## Install

```console
$ go install github.com/RomanosTrechlis/3d-linter/cli/cmd/3dl@latest
# or from a checkout:
$ cd cli && go build -o 3dl ./cmd/3dl
```

## Commands

| Command | What it does | Exit |
|---|---|---|
| `3dl check [path]` | Scan and gate the whole tree | 1 on Findings |
| `3dl check --diff BASE` | **Diff-only mode** — gate only lines added/modified since `BASE` (uses the merge base when one exists; untracked files count whole) | 1 on Findings |
| `3dl audit [path]` | Full-repo scan, advisory — always exits 0 (`--gate` to enforce) | 0 |
| `3dl init [path]` | Scaffold a starter `.glossary.yml` + GitHub Actions workflow | — |

Flags for `check`/`audit`: `--format text|json|sarif|github` · `--glossary FILE` (skip discovery, use one file) · `--strings` (also scan string literal contents — off by default).

## The glossary

`.glossary.yml`, discovered anywhere in the tree. In a monorepo, the **nearest ancestor** glossary owns a file — each project can keep its own language.

```yaml
scope:                      # optional, relative to this glossary's folder
  exclude: ["**/generated/**"]
terms:
  - use: practitioner       # the canonical term
    avoid: [user, customer] # forbidden synonyms — exact tokens, plus trivial
                            # plural/case variants (user ≙ Users). Never semantic.
    definition: A person following their daily practice.
    reason: '"user" is too generic and "customer" implies payment'
    scope:                  # optional per-term override
      exclude: ["auth/**"]
```

Matching covers identifiers in any language (`camelCase`, `snake_case`, `kebab-case`, `PascalCase` are split into words; compound forms match too, so avoiding `blacklist` also flags `black_list`) and prose in comments and Markdown. <!-- 3dl:allow blacklist -- the docs must name the example -->

> That HTML comment above is a live suppression — this repo gates itself with its own glossary.

## Suppressions

Inline, and the reason is mandatory — a bare allow does not suppress:

```go
id := profile.user // 3dl:allow user -- passport API field, not ours to rename
```

Suppressed Findings stay visible: they are counted in every report and carried in SARIF with their justification.

## What is deliberately skipped

- Directories: `.git`, `node_modules`, `vendor`, `testdata`, `dist`, `build`, `target`, `__pycache__`
- Files: binaries, files > 1 MiB, lockfiles, `*.min.*`, `*.svg`, `*.map`, and glossary files themselves
- String literal contents, unless `--strings` is passed (quote detection is line-local; multi-line strings pass through — per-language accuracy arrives with tree-sitter in v1)

## Determinism (N1)

Same repo state + same glossary + same base ref → byte-identical output, on any OS, offline. Locked by the golden test in `cmd/3dl/golden_test.go`; regenerate with `go test ./cmd/... -run TestGolden -update` after intended changes.

## Architecture

```
cmd/3dl/           CLI (cobra)
pkg/engine/        Rule Pack registry + run orchestration (the pivot)
pkg/rules/vocab/   Rule Pack #1: glossary load, avoid-list matching
pkg/scan/          shared token/file extraction (regex v0)
pkg/diff/          git base-ref diff mapping (Findings ∩ changed lines)
pkg/report/        text / json / SARIF / github renderers (rule-pack-agnostic)
```

A Rule Pack is one Go interface (`engine.Pack`); future packs (conventions, dependency boundaries — spec §9) plug into the same scanning and reporting without touching either.
