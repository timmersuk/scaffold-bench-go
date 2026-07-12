---
status: accepted
---

# Native Go AST checks for SB-25

After exploring an external TypeScript fact extractor, we decided that shelling out to `bun` from a Go evaluator to preserve a handful of upstream AST checks was the wrong trade-off. The inter-process contract (JSON blobs, exit codes, timeouts, repo-root path resolution) was brittle, and the approach produced an architectural smell: the Go server depending on a TypeScript runtime to avoid parsing code it can reasonably inspect itself.

We instead implement the three `ast_*` checks needed for SB-25 as native Go evaluator functions. We target scenario-test equivalence, not bit-for-bit parity with the upstream TypeScript compiler API. The Go checks must pass the upstream gold/broken gate fixtures for SB-25; they do not need to match upstream on contrived inputs we will never encounter.

## Decisions

- `ast_*` checks are native Go code inside `internal/runner/evaluator.go` (or a sibling `ast.go` file in the same package).
- The first implementation supports exactly the checks required for the SB-25 correctness rubric:
  - `ast_property_contains_call`
  - `ast_file_calls`
  - `ast_jsx_passes_prop`
- Each check receives the same `Input` and `params` as the other evaluator checks.
- Verification uses two layers:
  1. Synthetic evaluator unit tests with hand-written TSX snippets for quick regression.
  2. Ported upstream gate fixtures (`gold/` and `broken/` workspaces) used in an integration test that asserts the same pass/partial/fail outcomes.
- No CGO, no external parser library, no experimental pure-Go TypeScript parser. The checks use careful string and regex inspection tailored to the shapes the upstream functions look for.
- No new `ScenarioError` status or `Evaluation.ErrorKind` field; infrastructure problems during evaluation continue to use the existing failure path.

## Considered options

- **External TypeScript extractor** (ADR-0001): rejected because the inter-process contract and the runtime dependency on `bun`/`typescript` outweighed the fidelity benefit for the small number of checks we actually need.
- **Tree-sitter via Go bindings**: rejected to avoid a CGO dependency and because the set of required checks is small enough to make string/regex parsing proportionate.
- **Experimental pure-Go TypeScript parser**: rejected as immature and risky.
- **Porting all upstream `ast.ts` functions now**: rejected because #14 only requires validation of one representative `ast_*` boundary, and SB-25 uses only three concrete checks. We will add more functions when their scenarios are ported.

## Consequences

- ADR-0001 is superseded.
- The evaluator remains a single Go process with no extra dependency beyond the existing `bun` requirements for behavioral tests.
- New `ast_*` checks must be written in Go, tested with synthetic snippets, and validated against upstream gate fixtures when available.
- The Go checks must be maintained manually if upstream changes the scoring semantics for SB-25. Because we target scenario-test equivalence, minor upstream refactors that do not affect the gate fixtures will not require changes here.
