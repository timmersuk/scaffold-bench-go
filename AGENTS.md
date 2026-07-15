## Verification

Code is NOT done until verification passes, don't rely on isolated partial tests.

**Frontend:** `cd frontend && tsc --noEmit && bun run test:run`  
**Backend:** `go test ./...`

---

## Agent skills

### Issue tracker

Issues live in GitHub Issues for this repo; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context layout: read `CONTEXT.md` at the repo root and `docs/adr/` for decisions. See `docs/agents/domain.md`.
