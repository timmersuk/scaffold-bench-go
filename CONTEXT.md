# Scaffold Bench Context

Language for the Go-flavored scaffold-bench scenario format: neutral YAML manifests, fixture files, and evaluator-driven scoring.

## Language

**Scenario**:
A single coding task handed to a model, scored against a 10-point (or custom) rubric.
_Avoid_: Exercise, problem.

**Scenario Port**:
A translated version of an upstream TypeScript scenario from `1337hero/scaffold-bench` into this repo’s manifest/fixture format.

**Manifest**:
The hand-editable YAML file that declares a scenario’s metadata, workspace, rubric, and labels. The Go evaluator loads it at runtime.

**Fixture**:
A file shipped with a scenario: the starter workspace tree, the pristine reference copy, or a hidden behavioral test.

**Workspace Source**:
The starter files placed in the model’s work directory at the start of a run. For SB-01 this is `scenarios/SB-01/playground/`.

**Pristine Copy**:
The clean, un-mutated snapshot of the workspace used for diff-based scope and pattern checks. It is the reference for `function_equals_original`, `no_added_comments`, and `files_changed_only`.
_Avoid_: Golden workspace.

**Signal Type**:
The dominant correctness signal used by a scenario. It is metadata only; the evaluator still runs every configured rubric check.

**Behavioral Source of Truth**:
The behavior defined by the original TypeScript scenario in `1337hero/scaffold-bench`. When porting, the upstream fixtures and hidden tests are copied verbatim unless an explicit compatibility issue is declared.

**Upstream Provenance**:
A record that a field or fixture came from the original TypeScript scenario, kept for audit but not used by the Go evaluator.
_Avoid_: Source of truth.

**AST Check**:
A rubric check whose `type` begins with `ast_`, implemented as a native Go check that analyzes TypeScript or TSX source files to match the behavior of the upstream TypeScript benchmark. AST checks do not shell out; they are ordinary evaluator checks.

**Behavioral Source of Truth**:
The behavior defined by the original TypeScript scenario in `1337hero/scaffold-bench`. When porting, the upstream fixtures and hidden tests are copied verbatim unless an explicit compatibility issue is declared.

**Hidden Test**:
A test fixture kept outside the model workspace and copied into a temporary directory only during evaluation so the model cannot read the assertions.

**Model**:
A selectable Large Language Model available through an inference endpoint. In the API it is represented by an `id`, `source` (`local` or `remote`), `endpoint`, and an optional `requiresApiKey` flag.
_Avoid_: LLM, inference target.
