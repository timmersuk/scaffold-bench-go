# Run preflight: warmup gate + best-effort metadata survey

Before a run executes scenarios, we perform a warmup completion request (Level C readiness gate) that confirms the model is reachable and loads it into VRAM. This prevents model load time from contaminating scenario timing metrics. The warmup phase is exposed as a distinct `warming_up` run status with `model_warmup_started` / `model_warmup_finished` events.

All metadata (GPU backend/model/count/VRAM, quantization info, context size) is extracted from the warmup response — no additional HTTP calls. GPU detection relies on endpoint-reported info only; we do not shell out to platform tools. Quantization is parsed from the model file path (preferred) or model ID (fallback). Context size is extracted from `/v1/models` if the endpoint provides it, otherwise null.

If metadata extraction fails, the run continues with null fields. The metadata survey is best-effort; missing data does not invalidate the run.
