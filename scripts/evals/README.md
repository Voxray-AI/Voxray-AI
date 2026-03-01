# Evals

Go-native eval runner for voice pipeline scenarios. Runs a single pipeline (LLM only): injects a text prompt, collects the LLM response, and asserts using regex or substring.

## Usage

From repo root:

```sh
# Run all scenarios (uses config.json for API keys and provider)
go run ./cmd/evals -config scripts/evals/config/scenarios.json -voila-config config.json

# Run one scenario
go run ./cmd/evals -config scripts/evals/config/scenarios.json -voila-config config.json -scenario math_2_plus_2

# Verbose and custom output dir
go run ./cmd/evals -config scripts/evals/config/scenarios.json -voila-config config.json -out-dir scripts/evals/test-runs/my-run -v
```

## Environment

- **API keys**: Set in `config.json` under `api_keys` or via environment (e.g. `OPENAI_API_KEY`, `GROQ_API_KEY`). The runner uses the LLM provider and model from the voila config.
- **Config**: `-voila-config` points to the same JSON used by the voila server (provider, model, api_keys).

## Scenario format

`scenarios.json` contains a `scenarios` array. Each scenario has:

| Field | Description |
|-------|-------------|
| `name` | Identifier for the scenario |
| `prompt` | User message sent to the LLM (as TranscriptionFrame) |
| `expected_contains` | Substring that must appear in the response (case-insensitive) |
| `expected_pattern` | Regex (case-insensitive) that must match the response. Ignored if `expected_contains` is set |
| `timeout_secs` | Max time for the scenario (default 60) |
| `system_prompt` | Optional LLM system message override |

## Output

- Results are written to `scripts/evals/test-runs/<timestamp>/results.json` (or `-out-dir`).
- Summary is printed to stdout (pass/fail counts and per-scenario status).
- Exit code 1 if any scenario failed.

## Relation to upstream evals

The upstream scripts/evals are Python-based and use two bots in a Daily room (user bot + eval bot). This Go runner is a single-pipeline eval: one LLM run per scenario, no STT/TTS or transport. For full voice or multi-bot evals against a running Go service (e.g. WebSocket), you could extend the runner or use the upstream Python evals against a Go-backed endpoint.
