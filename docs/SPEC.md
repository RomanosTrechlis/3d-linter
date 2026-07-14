# 3D-Linter — Product Specification

**Working title:** `3d-linter` — 3D as in Domain-Driven Design (the final name is open item 1)
**Status:** v0 implemented; S/A/V2 items are roadmap

---

## 1. The job

> "Enforce our agreed domain vocabulary in code, comments, and docs — automatically, at PR time, without ceremony."

Teams that practice Domain-Driven Design (and any team that cares about naming) write a glossary of canonical terms with forbidden synonyms — then nothing enforces it, and the vocabulary silently drifts. 3D-Linter reads a glossary file from the repo and flags avoided terms in pull requests.

**The motivating bug:** a habit-tracking app defined "streak" ambiguously, and it got implemented as *consecutive days with a vice mark* — an app rewarding consecutive failures. It shipped, had to be renamed ("Engagement Streak"), and re-implemented. Vocabulary drift is a defect class, not a style nit.

## 2. The one architectural law

**The gate is deterministic; AI is advisory.** The merge-blocking check must be reproducible — same input, same verdict, every run, offline. No LLM ever sits in the blocking path (LLM verdicts are non-reproducible, per-run costly, and flaky — exactly what a CI gate must not be). AI features (§4.4) run out-of-band, produce *suggestions*, and are the paid tier.

## 3. Domain language

(This glossary is itself maintained in the tool's own format — see [.glossary.yml](../.glossary.yml); the repo eats its own dogfood.)

- **Glossary** — the versioned YAML file in the repo (`.glossary.yml`) defining Terms. The single source of truth. _Avoid:_ dictionary, vocabulary file.
- **Term** — one canonical concept: a `use` name, its `avoid` list, an optional `reason` and definition. _Avoid:_ word, entry.
- **Avoid list** — the forbidden synonyms for a Term (e.g. `use: practitioner, avoid: [user, customer]`). Matching is exact-token (+ trivial plural/casing variants), never semantic — semantics belong to the Advisor. _Avoid:_ banned words, blacklist.
- **Finding** — one flagged occurrence: file, line, matched token, the canonical Term, the reason. _Avoid:_ error, violation (findings in advisory surfaces are not failures).
- **Gate** — the deterministic PR check that fails when Findings exist in the diff. _Avoid:_ AI review.
- **Advisor** — the paid, out-of-band AI layer producing suggestions (bootstrap, semantic drift, maintenance reports). Never blocks. _Avoid:_ AI gate, auto-fix.
- **Diff-only mode** — the default: only tokens *introduced by the PR* are gated; pre-existing code and third-party references are never flagged. The central false-positive defense. _Avoid:_ full-repo scan (that's `audit`, an explicit opt-in command).
- **Rule Pack** — a pluggable family of deterministic checks the engine runs (vocabulary is Rule Pack #1; conventions and dependency boundaries come later — §9). Every Rule Pack obeys the §2 law and emits Findings through the same reporting pipeline. _Avoid:_ plugin (implies third-party extensibility, which is not promised), feature flag.
- **Suppression** — an inline opt-out (`// 3dl:allow user -- passport API field`) requiring a reason string. _Avoid:_ ignore (bare, reasonless).

## 4. Functional requirements

Priorities: **M** (v0 — implemented) / **S** (v1) / **A** (Advisor, paid) / **V2** (later).

### 4.1 Glossary
- **F1 (M)** `.glossary.yml` at repo root (path configurable): list of Terms (`use`, `avoid[]`, `definition?`, `reason?`), plus global `scope` (include/exclude globs) and per-term `scope` overrides.
- **F2 (M)** Multiple glossaries per repo (monorepo: nearest-ancestor glossary wins per file).
- **F3 (S)** Import/compat: read Contextive's glossary YAML — don't force teams that have one to migrate. Our additions (`avoid`, `reason`, `scope`) layer on top.

### 4.2 The scanner (deterministic core)
- **F4 (M)** Token extraction: identifiers split on `camelCase` / `snake_case` / `kebab-case` / `PascalCase`; prose words in comments, strings (opt-in), and Markdown. Language-agnostic regex extraction in v0 — no per-language AST yet.
- **F5 (M)** Matching: exact token vs. Avoid list, case-insensitive, plus trivial plurals (`user`/`users`). **No stemming beyond that, no fuzzy, no embeddings** — precision over recall is the v0 religion.
- **F6 (M)** Diff-only mode: given a base ref, gate only added/modified lines. `3dl audit` runs full-repo (advisory exit code by default).
- **F7 (M)** Suppressions with mandatory reason; findings report suppression counts so they stay visible.
- **F8 (S)** Declaration-aware scanning via tree-sitter for the top languages (Go, TS/JS, Python, Java, C#): flag only where a name is *declared/introduced*, not where an external API is referenced. The big precision upgrade of v1.
- **F9 (S)** `3dl fix` — safe mechanical rename suggestions as a patch (never auto-applied).

### 4.3 Surfaces
- **F10 (M)** CLI: `3dl check [--diff base] [--format text|json|sarif]`. Single static binary (Go). Exit 0/1. Runs fully offline.
- **F11 (M)** GitHub Action: wraps the CLI, annotates the PR with file/line Findings. No server component; everything executes in the runner.
- **F12 (S)** Pre-commit hook recipe; GitLab CI recipe (same binary, SARIF out); Jenkins to follow.
- **F13 (V2)** IDE surface — only if demand appears; Contextive already serves hover/autocomplete well and F3 makes coexistence natural: Contextive teaches the language in the editor, 3D-Linter enforces it at the gate.

### 4.4 The Advisor (paid, out-of-band)
- **F14 (A)** `3dl bootstrap` — points an LLM at the codebase and *drafts* the glossary: candidate canonical terms, detected synonym clusters, suggested Avoid lists. Output is a `.glossary.yml` PR for humans to edit.
- **F15 (A)** Semantic drift suggestions: PR *comments* (never check failures) flagging unlisted near-synonyms of glossary Terms.
- **F16 (A)** Maintenance report: periodic "new domain terms appeared recently — canonize or ban" summary.
- **F17 (A)** All Advisor features are **BYO-API-key** (Anthropic/OpenAI/compatible endpoint). Your code never transits our infrastructure; the free Gate never gains a network dependency. License keys are verifiable offline — no license server, no accounts.

## 5. Non-functional requirements

- **N1 — Determinism:** same repo state + same glossary + same base ref → byte-identical Findings, offline, on any OS. CI-tested with a golden-output lock.
- **N2 — Precision over recall:** the acceptance bar is a false-positive rate low enough that the tool survives week one, validated by dogfooding on real repos with real glossaries (including this one).
- **N3 — Speed:** diff-mode check < 1s on a typical PR; full audit linear and parallel. A slow linter is a deleted linter.
- **N4 — Zero ops:** no server, no telemetry, no accounts. The free tool makes no network calls at all; the Advisor calls only the user's own LLM endpoint.
- **N5 — No ceremony:** `3dl init` scaffolds a starter glossary and Action workflow in one command; a team is gated within 15 minutes.

## 6. Architecture

The engine is a **conformance engine running Rule Packs**: sources are parsed once into a scan model, Rule Packs consume it and emit Findings, renderers consume Findings. Adding a Rule Pack later touches neither the scanning nor the reporting side.

```
cmd/3dl/           CLI (cobra)
pkg/engine/        rule-pack registry + run orchestration (the pivot)
pkg/rules/vocab/   RULE PACK #1: glossary load (+ contextive import, F3),
                   avoid-list matching, plurals, casing
pkg/rules/...      future packs (§9): conventions/, deps/ — NOT in v0
pkg/scan/          shared token/file extraction (regex v0; treesitter/ v1
                   behind the same interface)
pkg/diff/          git base-ref diff mapping (findings ∩ changed lines)
pkg/report/        text / json / SARIF renderers (rule-pack-agnostic)
action/            thin composite GitHub Action wrapping the binary
```

Go single binary: cross-platform static distribution and the deterministic-core philosophy. The Rule Pack abstraction costs one Go interface in v0 (`Pack: Check(scanModel) []Finding` + a config key per pack) — vocabulary is the only pack that exists today.

## 7. Out of scope

Semantic matching in the Gate (Advisor-only, forever — see §2) · auto-applied renames · a hosted dashboard/server · IDE plugin in v0/v1 (F13) · natural-language prose *style* linting (Vale's job; we lint *vocabulary*, not style) · non-git VCS · any Rule Pack beyond vocabulary in v0/v1.

## 8. Open items

1. **Name.** Candidates should say "vocabulary, enforced" — decide before the Action is published (marketplace renames are painful).
2. **Glossary format final call:** own minimal schema with Contextive import (F3), or adopt Contextive's schema outright and extend.
3. **SARIF vs. check-run API** for annotations. v0 resolves this with a third option — a `github` output format emitting workflow commands (zero extra permissions); SARIF output remains available for code-scanning uploads.

## 9. Expansion ladder (roadmap, demand-gated)

The long-range shape is an ArchUnit-class conformance tool: many deterministic rule families, one gate. Each rung only starts when the previous one has earned it — the engine architecture (§6) makes each an addition, never a rewrite.

| Rung | Rule Pack | Status |
|---|---|---|
| **1** | **Vocabulary** (this spec, F1–F13) | v0 shipped; v1 on resonance |
| **2** | **Conventions** — naming patterns ("handlers end in `Handler`"), file/folder structure rules ("no SQL outside `repositories/`"). Regex/path-level, language-agnostic | gated on rung 1 adoption |
| **3** | **Dependency boundaries, one language: Go** — import-graph rules, cycle detection, via `go/packages` | gated on demonstrated pull |
| **4** | **Further languages** (TS, Python, …) — each a new extraction backend behind the same rules | strictly per-language demand |
