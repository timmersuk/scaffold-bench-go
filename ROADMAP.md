# Roadmap: scaffold-bench-go

This document tracks how much of the Go/Vite port of scaffold-bench is implemented. It is derived from the wayfinder map ([#1](https://github.com/timmersuk/scaffold-bench-go/issues/1)) and the open implementation backlog.

## Destination

A single-binary Go + Vite port of [scaffold-bench](https://github.com/1337hero/scaffold-bench). The Go server provides the benchmark REST API and SSE event stream; the React frontend in `frontend/` is built into `internal/web/dist` and embedded in the binary.

## Progress overview

| Area | Status | Notes |
|------|--------|-------|
| Architecture & project skeleton | ✅ Done | Go 1.26, stdlib HTTP, SQLite, Vite+React+Tailwind, Makefile, Docker, CI |
| Domain model & REST API | ✅ Designed | Core entities and `/api/*` routes defined in ADRs / issues |
| SQLite schema & persistence | ✅ Done | Migrations, runs/scenario_runs/events tables, WAL |
| Scenario manifest schema | ✅ Done | `docs/design/scenario-manifest.md`, YAML/JSON loader, Go evaluator interface |
| Run engine & evaluator | 🚧 Partial | Core checks complete; `requires` enforcement and trace-semantic fixes landed (#24, #25). Native Go AST checks implemented for SB-25 (#14, ADR-0002). Remaining gaps: parallel tool execution (#17), preflight metadata (#18). |
| Scenarios ported | 🚧 In progress | SB-01 and SB-25 ported and validated against golden workspaces; remaining 48 scenarios not started |
| Frontend wiring | 🚧 Partial | `/api/scenarios`, `/api/models`, and SSE run stream are wired; Dashboard view implemented in PR #29. RunHistory and OneShotLab remain placeholders. |
| One-shot lab | ❌ Not started | API stubs only (`/api/oneshot/*` return empty) |
| Reports / leaderboard | ❌ Not started | `/api/report/data` returns empty skeleton |
| Frontend tests | ❌ Missing | No Vitest / React Testing Library setup |

## Decisions captured

All closed design issues from the wayfinder map:

- [x] [#2 Define MVP scope and feature cut for the Go rewrite](https://github.com/timmersuk/scaffold-bench-go/issues/2)
- [x] [#3 Choose scenario execution strategy](https://github.com/timmersuk/scaffold-bench-go/issues/3)
- [x] [#4 Select the Go HTTP stack and project conventions](https://github.com/timmersuk/scaffold-bench-go/issues/4)
- [x] [#5 Design the domain model and REST API contract](https://github.com/timmersuk/scaffold-bench-go/issues/5)
- [x] [#6 Design the SQLite schema and persistence layer](https://github.com/timmersuk/scaffold-bench-go/issues/6)
- [x] [#7 Prototype the frontend architecture and main views](https://github.com/timmersuk/scaffold-bench-go/issues/7)
- [x] [#8 Set up the initial project skeleton and build pipeline](https://github.com/timmersuk/scaffold-bench-go/issues/8)
- [x] [#9 Design the neutral scenario manifest schema and evaluator interface](https://github.com/timmersuk/scaffold-bench-go/issues/9)

## Implementation backlog

### Run engine & evaluator

- [x] [#14 Add an AST plugin boundary for ast_* rubric checks](https://github.com/timmersuk/scaffold-bench-go/issues/14) — implemented as native Go checks for SB-25 (ADR-0002)
- [ ] [#17 Support parallel tool execution and tool-call hooks](https://github.com/timmersuk/scaffold-bench-go/issues/17)
- [ ] [#18 Add run metadata and preflight checks](https://github.com/timmersuk/scaffold-bench-go/issues/18)
- [x] [#24 Fix trace_read_before_edit to require an edit/write](https://github.com/timmersuk/scaffold-bench-go/issues/24)
- [x] [#25 Runner engine does not enforce manifest requires list](https://github.com/timmersuk/scaffold-bench-go/issues/25)

### Scenarios

- [x] [#16 Port the first real scenario (SB-01) and validate its score](https://github.com/timmersuk/scaffold-bench-go/issues/16)
- [x] Port SB-25 and validate native AST checks against upstream gate fixtures
- [ ] Port SB-02 through SB-24, SB-26 through SB-50 (not yet ticketed in detail)

### API / models

- [x] [#15 Expose scenarios and models to the frontend](https://github.com/timmersuk/scaffold-bench-go/issues/15)
- [ ] [#21 Query BENCH_REMOTE_ENDPOINT /v1/models for dynamic remote model list](https://github.com/timmersuk/scaffold-bench-go/issues/21)
- [ ] [#22 Add display names for models in /api/models response](https://github.com/timmersuk/scaffold-bench-go/issues/22)
- [ ] [#23 Reuse HTTP client for local /v1/models discovery](https://github.com/timmersuk/scaffold-bench-go/issues/23)
- [ ] [#34 Add runtime configuration persistence with REST API and frontend UI](https://github.com/timmersuk/scaffold-bench-go/issues/34) — persists endpoint/model list settings in a JSON file in the data folder, editable via the UI only when no run is active

### Frontend & UX

- [x] Wire Dashboard to `/api/scenarios`, `/api/models`, and SSE run stream — implemented in PR #29
- [ ] Wire RunHistory to stored runs and report data
- [ ] Implement OneShotLab views and connect to `/api/oneshot/*`
- [ ] Add Vitest + React Testing Library and component tests

### Infrastructure

- [x] Dockerfile runtime image has `bun` and `golang-go` — added in PR #26

## Out of scope

From [#1](https://github.com/timmersuk/scaffold-bench-go/issues/1):

- Rewriting the 50 scenario definitions beyond the neutral manifest + Go evaluator work.
- Bun/TypeScript-based orchestration of the run loop.
- Authentication for the web UI or API.
- New benchmark dimensions, new scoring rubrics, or new model providers.
- Bubblewrap sandboxing of scenario workspaces.

## How to update this roadmap

1. Create or close an issue for the work.
2. Update the checklist and status table in this file.
3. Reference this document in related PRs so the roadmap stays current.

## Tracking links

- [GitHub Issues](https://github.com/timmersuk/scaffold-bench-go/issues)
- [Project board: scaffold-bench-go roadmap](https://github.com/users/timmersuk/projects/2)
