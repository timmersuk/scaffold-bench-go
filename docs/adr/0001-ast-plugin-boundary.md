---
status: superseded by ADR-0002
---

# AST plugin boundary via external TypeScript fact extractor

We need to run AST-dependent rubric checks while keeping the evaluator in Go. The upstream TypeScript benchmark already has a set of exact AST checks in `lib/scenarios/_shared/evaluators/ast.ts` that use the TypeScript compiler API. Reimplementing those semantics in Go would be fragile and would drift from the original scoring, so we decided to delegate `ast_*` checks to an external AST fact extractor. The first extractor is a TypeScript tool invoked via `bun`; it wraps the upstream functions directly. The Go evaluator stays in charge of scoring and only treats extractor infrastructure failures as scenario errors.

## Decisions

- Keep the `ast_*` check-type prefix for upstream compatibility; the extractor maps `ast_snake_case` to the upstream camelCase function by convention.
- Invoke the extractor as a separate process per check: `bun run <repo-root>/tools/ast-extractor.ts <checkType> '<json-params>'`.
- Run the extractor with the scenario workspace root as the working directory; the JSON params carry the workspace-relative file path.
- Use a JSON blob as the single CLI argument so the contract is self-describing and new params do not change the CLI shape.
- Adapters inside the extractor read named JSON params and call the upstream functions explicitly. This decouples the manifest from the upstream argument order.
- Repo-level tool: `tools/ast-extractor.ts` lives in the repo root; a root `package.json` supplies the `typescript` dependency.
- The evaluator discovers the repo root from the Go source/binary layout at runtime. Deployments must keep the binary next to the repo tree until we embed the logic in Go.
- Exit-code contract:
  - `0` = semantic pass; stdout is the detail string.
  - `1` = semantic failure; stdout/stderr is the detail string.
  - `2` = extractor infrastructure error; the scenario run gets `ErrorKind: "extractor_error"` and `Status: "error"`.
- Support `params.timeout_ms`; default is 10 seconds.
- Scenarios that use `ast_*` checks add `requires: ["bun"]`. A missing `bun` causes the scenario to be skipped, reusing the existing requirement machinery.
- The first implementation targets the full SB-25 correctness rubric: `ast_property_contains_call`, `ast_file_calls`, and `ast_jsx_passes_prop`.
- Extend the evaluation model so that `Evaluation` carries `ErrorKind` and `Error`, and add a `ScenarioError` status for infrastructure failures.

## Considered options

- **Embedded Go parser** (tree-sitter, oxc, swc): would remove the external process, but none match the TypeScript compiler API exactly, so we would lose scoring fidelity. Rejected because faithfulness to upstream behavior is the top priority.
- **Batch extractor invocations**: one process per scenario with a JSON request map. Rejected for the first implementation because one process per check is simpler to debug and the AST checks are not yet numerous enough for start-up cost to dominate.
- **Positional CLI arguments**: rejected in favor of a single JSON blob because manifest params can be optional and named, and positional ordering is hard to keep stable.
- **Explicit handler registry vs naming convention**: we chose a naming convention to keep the extractor tiny, accepting the risk that upstream function renames break us. Tests that exercise every supported `ast_*` check will catch those regressions.

## Consequences

- Deployments must keep the Go binary and the repo tree together; moving the binary alone will break AST checks.
- The Docker image and CI must install `bun` and run `bun install` at the repo root so `typescript` is available.
- New AST checks require three things: an upstream function, an adapter in `tools/ast-extractor.ts`, and a test that pins the mapping.
- The `extractor_error` status gives us a clean way to distinguish "the model failed" from "we could not score the model."
- This boundary is intentionally temporary: a future ticket will move the AST logic into the Go server process to remove the `bun`/TypeScript runtime dependency.
