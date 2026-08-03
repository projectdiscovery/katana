# Contributing to Katana

Thank you for helping improve Katana. This guide explains how to propose, implement, test, document, and submit changes to the project.

## Before You Start

Katana uses a discussion-first workflow. Start with a [GitHub Discussion](https://github.com/projectdiscovery/katana/discussions) for bugs, features, and questions. Maintainers convert confirmed, actionable work into issues. Pull requests must be associated with the relevant issue or approved feature request.

Do not publicly report security vulnerabilities. Follow the private reporting process in [SECURITY.md](SECURITY.md).

## Development Setup

You need:

- The Go version declared in [`go.mod`](go.mod).
- Git.
- `make` for the repository shortcuts.
- `golangci-lint` when running lint checks locally.
- Chrome or Chromium when working on headless crawling.

Clone the repository and create a branch from `dev`:

```console
git clone https://github.com/projectdiscovery/katana.git
cd katana
git checkout dev
git pull --ff-only
git checkout -b <type>/<short-description>
```

Use a focused branch name such as `fix/redirect-scope` or `feature/crawl-summary`.

## Repository Map

Use this map to find the right place for a change:

| Path | Responsibility |
| --- | --- |
| `cmd/katana` | CLI entry point and flags |
| `internal/runner` | Option validation, engine selection, input orchestration, and lifecycle |
| `pkg/engine/common` | Behavior shared by crawler engines |
| `pkg/engine/standard` | HTTP crawler |
| `pkg/engine/headless` | Browser-based crawler |
| `pkg/engine/hybrid` | Combined standard and headless crawling |
| `pkg/engine/parser` | Response and known-file parsing |
| `pkg/navigation` | Request and response models |
| `pkg/output` | Result filtering, formatting, and storage |
| `pkg/similarity` | Page-content similarity filtering |
| `pkg/types` | Public and internal configuration types |
| `pkg/utils` | Reusable URL, scope, queue, filter, and form helpers |
| `internal/testutils` | Shared test infrastructure |
| `integration_tests` | End-to-end CLI tests |

Prefer changing the narrowest applicable package. Shared behavior belongs in `pkg/engine/common` only when multiple engines genuinely need it.

## Make One Focused Change

Keep each pull request small enough to review independently:

1. Describe the current behavior and desired behavior.
2. Identify the smallest package that owns that behavior.
3. Add or update a test that demonstrates the expected result.
4. Implement the change without unrelated refactoring.
5. Update user-facing documentation when flags, output, configuration, or public APIs change.
6. Run the checks appropriate for the affected area.

Avoid mixing dependency upgrades, formatting-only edits, refactors, and behavior changes in one pull request.

## Guidance by Change Type

### Bug fixes

- Add a regression test that fails before the fix and passes afterward.
- Preserve existing behavior outside the reported case.
- Include reproduction steps and explain the root cause in the pull request.

### Features and improvements

- Agree on the user-facing behavior in a Discussion or issue before implementation.
- Keep defaults backward compatible whenever possible.
- Document new CLI flags in `README.md` and public options in `pkg/types`.
- Cover enabled, disabled, valid, and invalid configurations where applicable.

### CLI and output changes

- Keep normal, verbose, JSONL, and silent modes in mind.
- Treat output formats as compatibility-sensitive; scripts may depend on them.
- Test singular, plural, empty, error, and filtered-result cases when changing summaries.
- Avoid writing diagnostic messages to streams intended for machine-readable output.

### Crawler-engine changes

- Consider standard, headless, and hybrid behavior.
- Preserve scope, depth, uniqueness, rate-limit, cancellation, and response-size guarantees.
- Do not perform unbounded reads or create goroutines without an explicit shutdown path.
- Test concurrent code with the race detector.

### Public API changes

- Prefer additive changes to breaking changes.
- Add comments to exported identifiers.
- Keep CLI and library behavior consistent where they expose the same option.
- Include a library usage example when behavior is not self-evident.

### Documentation-only changes

- Verify commands and links.
- Keep examples concise and safe to copy.
- Update nearby documentation rather than duplicating conflicting instructions.

### Dependency changes

- Explain why the dependency is needed.
- Prefer the standard library or an existing dependency when practical.
- Run `go mod tidy` and include only the expected `go.mod` and `go.sum` changes.
- Review licensing and security implications before introducing a new module.

## Code Standards

- Format Go files with `gofmt`.
- Follow standard Go naming and error-handling conventions.
- Wrap errors with useful operation context.
- Keep imports at file scope; do not hide import failures behind recovery logic.
- Avoid package-level mutable state when state can belong to a runner, session, or writer.
- Make resource ownership clear and make cleanup safe.
- Use a context-aware wait when cancellation should interrupt work.
- Write comments that explain intent or constraints rather than restating code.

## Tests

Start with the narrowest test, then expand validation as needed.

```console
# Test the package you changed
go test ./path/to/package

# Run all unit tests
make test

# Check concurrent changes
go test -race ./path/to/package

# Run lint checks
make lint

# Build the CLI
make build

# Run integration tests when CLI or crawl behavior changes
make integration
```

Before committing, also check formatting and whitespace:

```console
gofmt -w <changed-go-files>
git diff --check
```

Headless and integration tests may require Chrome, network access, or other environment-specific dependencies. If a check cannot run, state the exact limitation in the pull request; do not describe an unrun check as passing.

## Commit Guidelines

Write concise, imperative commit subjects. A conventional prefix is encouraged:

```text
fix(parser): preserve base URL during redirect
feat(runner): report crawl target count
test(output): cover filtered JSON results
docs: add contribution workflow
```

Keep commits reviewable. Each commit should build and should not contain generated binaries, local output, credentials, browser profiles, or editor files.

## Pull Requests

Open pull requests against the `dev` branch and complete `.github/PULL_REQUEST_TEMPLATE.md`.

The description should include:

1. **Proposed changes**: what changed and why.
2. **Related work**: the approved issue or feature request.
3. **Proof**: exact commands, test output, logs, or screenshots that demonstrate the change.
4. **Compatibility**: effects on CLI output, JSON fields, configuration, stored state, and public APIs.
5. **Documentation**: files updated, or why no documentation change is required.

Before requesting review, confirm that:

- The pull request targets `dev`.
- The diff contains only relevant changes.
- New behavior has tests.
- User-facing behavior is documented.
- Formatting, tests, lint, and applicable integration checks pass.
- No secrets or sensitive crawl data appear in code, fixtures, logs, or screenshots.

## Review and Follow-up

Respond to review comments with either a code update or a short explanation. Resolve the underlying issue rather than only the example mentioned by a reviewer. When behavior changes during review, update tests and documentation in the same pull request.

Maintainers may ask for a change to be split when it combines unrelated work or is too large to validate safely.
