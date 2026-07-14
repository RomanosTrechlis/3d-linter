# 3D-Linter

> Working title — 3D as in Domain-Driven Design; the final name is open item 1 in [docs/SPEC.md](docs/SPEC.md).

**Enforce your agreed domain vocabulary in code, comments, and docs — automatically, at PR time, without ceremony.**

Teams that practice Domain-Driven Design write a glossary of canonical terms with forbidden synonyms — then nothing enforces it, and the vocabulary silently drifts. 3D-Linter reads a `.glossary.yml` from the repo and fails pull requests that introduce avoided terms.

The one architectural law: **the gate is deterministic; AI is advisory.** Same input, same verdict, every run, offline. No LLM ever sits in the blocking path.

## Repo layout

One product, one engine, many surfaces — each surface is a top-level folder wrapping the same CLI:

| Folder | What it is |
|---|---|
| [`cli/`](cli/) | The `3dl` Go CLI — the whole engine lives here; every other surface is a thin wrapper |
| [`github-action/`](github-action/) | Composite GitHub Action running the gate with inline PR annotations |
| [`website/`](website/) | Static site explaining the tool (no build step — deploy the folder as-is) |
| [`docs/`](docs/) | [The product spec](docs/SPEC.md) — the behavioral contract the code implements |

Future surfaces (`gitlab/`, `jenkins/`, `pre-commit/`) land as sibling folders wrapping the same binary — SARIF/JSON output already covers most of what they need.

## Quickstart

```console
$ go install github.com/RomanosTrechlis/3d-linter/cli/cmd/3dl@latest
$ 3dl init            # scaffolds .glossary.yml + a GitHub Actions workflow
$ 3dl check --diff main
```

See [cli/README.md](cli/README.md) for the full command reference and glossary format, and [github-action/README.md](github-action/README.md) for CI setup.

This repo gates itself: [.glossary.yml](.glossary.yml) holds the project's own domain language (Term, Finding, Gate, Advisor, Rule Pack…), and CI runs the action on every PR.

## Status

v0 — every "M" requirement in [docs/SPEC.md](docs/SPEC.md): glossary loading (multiple per repo, nearest ancestor wins), regex token scanner, diff-only gating, reasoned suppressions, `text|json|sarif|github` output, GitHub Action, golden-output determinism test. Home: [github.com/RomanosTrechlis/3d-linter](https://github.com/RomanosTrechlis/3d-linter). The project name itself is still open item 1 — settle it before the Action is published, since marketplace renames are painful.

Deliberately **not** built yet, per the spec's priority ladder: Contextive import (F3), tree-sitter declaration awareness (F8), `3dl fix` (F9), pre-commit/GitLab recipes (F12), IDE surface (F13), and everything Advisor (F14–F17).

## Development

```console
$ cd cli
$ go test ./...       # includes the golden-output lock (N1)
$ go test ./cmd/... -run TestGolden -update   # regenerate goldens after intended changes
```

## License

[Apache-2.0](LICENSE). The deterministic Gate — everything in this repo — is free forever.
