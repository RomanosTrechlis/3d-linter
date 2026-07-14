# 3D-Linter GitHub Action

Thin composite action: builds the CLI in the runner and gates the PR. No server component; nothing leaves the runner (F11/N4).

## Usage

```yaml
name: vocabulary gate
on: pull_request
jobs:
  3dl:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # full history so diff-only mode can find the merge base
      - uses: RomanosTrechlis/3d-linter/github-action@master
```

And a `.glossary.yml` at the repo root:

```yaml
terms:
  - use: practitioner # your canonical term
    avoid: [user, customer]
    reason: '"user" is too generic and "customer" implies payment'
```

`3dl init` writes both files for you. The action builds the CLI inside the runner, so there is no separate binary to fetch or trust.

With the default shallow checkout the action still works: it falls back to a direct diff against the fetched base branch, which on GitHub's premerged PR checkout yields the same added lines. `fetch-depth: 0` is simply the most precise setup.

## Inputs

| Input | Default | Notes |
|---|---|---|
| `base` | the PR base branch | Diff-only mode gates lines added/modified since this ref. Empty (e.g. push builds) → the whole tree is gated. |
| `format` | `github` | `github` renders inline PR annotations via workflow commands — no code-scanning permissions needed. Use `sarif` if you prefer uploading to code scanning. |
| `args` | — | Extra `3dl check` arguments, e.g. `--strings`. |

Findings exit the step with code 1, which fails the check — that is the gate.

## Publishing note

Marketplace listing requires an `action.yml` at the repo root; when the name (spec open item 1) is settled, add a root-level shim that delegates here, or split this folder into its own repo. Until then, reference the action by path: `RomanosTrechlis/3d-linter/github-action@master`.

## Other CI systems

GitLab CI, Jenkins, and pre-commit surfaces are planned as sibling folders. The same binary already serves them today: run `3dl check --diff "$BASE" --format sarif` in any runner and feed the SARIF wherever your platform wants it.
