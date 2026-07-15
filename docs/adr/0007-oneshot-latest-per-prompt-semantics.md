# One-Shot Lab uses latest-per-prompt result semantics

The One-Shot Lab stores results for LabPrompts using latest-per-prompt semantics: rerunning a prompt replaces its previous result without affecting other prompts' results. The primary key is `prompt_id`, not `(run_id, prompt_id)`.

This matches the upstream scaffold-bench implementation and reflects the One-Shot Lab's purpose as a qualitative vibe check rather than a historical benchmark. Users care about the current output, not trends over time. If historical tracking becomes necessary, a separate `oneshot_history` table can be added without breaking the main flow.
