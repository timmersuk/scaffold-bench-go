# Closed enums for Category and Difficulty

Scenario manifests declare `category` and `difficulty` fields. The manifest loader validates these against closed enum sets rather than accepting arbitrary strings.

Allowed values:
- **Category**: `surgical-edit`, `scope-discipline`, `verify-and-repair`, `implementation`, `read-only-analysis`, `responsiveness`, `long-context`
- **Difficulty**: `low`, `medium`, `high`

A manifest with a value outside these sets fails validation at load time.

This prevents scenario authors from introducing new categories or difficulty levels by freestyling YAML strings. The report's leaderboard columns (category breakdown, tier breakdown) depend on a fixed set of values. Allowing arbitrary strings would break UI parity with upstream and require dynamic column generation.

The trade-off is less flexibility — adding a new category or difficulty requires a code change and migration, not just a manifest update. This is acceptable because the benchmark's task taxonomy is stable and upstream-compatible.
