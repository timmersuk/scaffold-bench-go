---
status: accepted
---

# Normalize line endings in scenario fixtures and evaluation workspaces

Scenario fixtures are often edited on Linux and checked out on Windows, where Git can convert LF line endings to CRLF. This caused the SB-01 golden-workspace test to fail: the test's replacement strings used LF, the fixture on disk used CRLF, and content-based checks (function extraction, comment comparison, workspace diff) reported false differences.

We decided the framework should treat line endings as an environmental artifact, not as a meaningful difference. The repository keeps fixture files as LF, and the evaluator normalizes workspaces to LF before scoring.

## Decisions

- Add a `.gitattributes` rule (`scenarios/** text eol=lf`) so fixture files check out as LF on all platforms.
- Renormalize existing scenario fixture files to LF.
- The evaluator canonicalizes the mutated workspace to LF before running checks (`canonicalizeWorkspaceText`).
- Workspace diff (`walkFiles`) normalizes CRLF to LF when comparing current and pristine copies.
- Content-comparison checks (`function_equals_original`, `no_added_comments`) normalize both sides before extracting functions or comments.

## Considered options

- **Make each evaluator check CRLF-tolerant**: rejected because every new check would have to remember to normalize, and it is easy to miss one.
- **Do nothing and rely on Windows CI running with `core.autocrlf=false`**: rejected because it makes local Windows development fragile and leaves the diff/check comparison semantics platform-dependent.

## Consequences

- Windows developers can run the full test suite without manually fixing fixture line endings.
- A model that only changes line endings will not be scored as having modified the corresponding file. We consider this correct: line-ending-only changes are not meaningful for the benchmark.
- Fixture files under `scenarios/` are now expected to be LF. Any CRLF fixture introduced in the future will be renormalized by Git on commit.
