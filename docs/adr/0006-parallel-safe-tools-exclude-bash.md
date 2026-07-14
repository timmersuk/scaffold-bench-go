# Parallel-safe tool set excludes `bash`

Upstream scaffold-bench includes `bash` in the parallel-safe tool set with a concurrency limit (`MAX_PARALLEL_BASH_CALLS = 2`). We exclude `bash` from the parallel-safe set, restricting parallel execution to `read` and `ls` only.

The `bash` tool executes arbitrary shell commands and is not truly read-only — a model could emit `bash` calls that mutate state, create files, or have side effects. Running such calls concurrently introduces race conditions and unpredictable behavior. By limiting parallel execution to genuinely read-only tools, we maintain safety and predictability at the cost of some performance in mixed batches.

## Considered Options

- **Match upstream exactly** — include `bash` with a concurrency limit of 2
- **Exclude `bash` from parallel-safe set** — only `read` and `ls` run in parallel
- **Add a static analysis pass** — detect whether a `bash` command is read-only before allowing parallel execution

We chose the second option because static analysis of shell commands is fragile and error-prone, and matching upstream would sacrifice safety for feature parity.
