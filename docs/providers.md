# LLM providers

Driftlock calls an LLM to (1) judge whether documentation still matches a structural code change and (2) rewrite the affected doc sections when it does not. This page shows how to configure each supported provider and how to keep costs negligible.

## Two adapters, several drivers

Under the hood Driftlock has exactly two adapters:

- **OpenAI-compatible** — used by the drivers `openai-compatible`, `groq`, `openrouter`, `deepseek`, and `vllm`. These are all the same adapter; the different names are just labels for clarity. Any provider that speaks the OpenAI chat-completions API works here.
- **Ollama** — the native adapter for a local [Ollama](https://ollama.com) server, selected with `driver = "ollama"`.

## The `endpoint` is the FULL URL

The single most important rule: **`endpoint` must be the complete chat-completions URL.** Driftlock POSTs to *exactly* the URL you provide and appends nothing. `https://openrouter.ai/api/v1` is wrong; `https://openrouter.ai/api/v1/chat/completions` is right.

## Recommended settings for all providers

```toml
[llm.options]
temperature = 0.0    # deterministic verdicts → better caching, stable rewrites
max_tokens = 4096    # enough headroom for chunked rewrites
```

`temperature = 0.0` is strongly recommended: it makes verdicts reproducible and maximizes [cache](./caching.md) hits.

---

## OpenRouter

A gateway to many models (including cheap DeepSeek variants). Get a key at <https://openrouter.ai/keys>.

```toml
[llm]
driver = "openrouter"     # or "openai-compatible"
endpoint = "https://openrouter.ai/api/v1/chat/completions"
model = "deepseek/deepseek-chat"
api_key = "${DRIFTLOCK_API_KEY}"
[llm.options]
temperature = 0.0
max_tokens = 4096
```

---

## Groq

Very fast inference for open models. Get a key at <https://console.groq.com>.

```toml
[llm]
driver = "groq"           # or "openai-compatible"
endpoint = "https://api.groq.com/openai/v1/chat/completions"
model = "llama-3.3-70b-versatile"
api_key = "${DRIFTLOCK_API_KEY}"
[llm.options]
temperature = 0.0
max_tokens = 4096
```

---

## DeepSeek

DeepSeek's first-party API — inexpensive and strong at code. Get a key at <https://platform.deepseek.com>.

```toml
[llm]
driver = "deepseek"       # or "openai-compatible"
endpoint = "https://api.deepseek.com/chat/completions"
model = "deepseek-chat"
api_key = "${DRIFTLOCK_API_KEY}"
[llm.options]
temperature = 0.0
max_tokens = 4096
```

---

## Together AI

Together exposes an OpenAI-compatible endpoint; use the `openai-compatible` driver. Get a key at <https://api.together.xyz>.

```toml
[llm]
driver = "openai-compatible"
endpoint = "https://api.together.xyz/v1/chat/completions"
model = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
api_key = "${DRIFTLOCK_API_KEY}"
[llm.options]
temperature = 0.0
max_tokens = 4096
```

---

## vLLM (self-hosted)

If you serve a model with [vLLM](https://github.com/vllm-project/vllm), it exposes an OpenAI-compatible server. Point the endpoint at your host's chat-completions path.

```toml
[llm]
driver = "vllm"           # or "openai-compatible"
endpoint = "http://localhost:8000/v1/chat/completions"
model = "Qwen/Qwen2.5-Coder-32B-Instruct"
api_key = "${DRIFTLOCK_API_KEY}"   # only if your server requires one
[llm.options]
temperature = 0.0
max_tokens = 4096
```

The `api_key` is sent as `Authorization: Bearer <key>`; omit or leave empty if your server is unauthenticated.

---

## Ollama (local, native)

For a fully local, zero-cost setup, run [Ollama](https://ollama.com) and use the native driver.

```toml
[llm]
driver = "ollama"
endpoint = "http://localhost:11434"
model = "codestral:22b"
# no api_key needed
[llm.options]
temperature = 0.0
```

Pull the model first:

```bash
ollama pull codestral:22b
```

Ollama is the built-in default (`driver = "ollama"`, `endpoint = "http://localhost:11434"`, `model = "codestral:22b"`). Good code models for drift judging include `codestral:22b`, `qwen2.5-coder`, and `llama3.1`.

---

## Cost and economics

Driftlock is engineered to keep token spend tiny. Four mechanisms compound:

1. **Structural-only diffs.** By default (`include_full_diff = false`) Driftlock sends only the **added/removed/modified signatures**, not the whole git diff. Body edits, comments, and formatting never reach the model. This is usually a few lines even for large commits.

2. **Smart chunking.** For each drifted symbol, Driftlock extracts only the documentation **section(s) mentioning that symbol** and sends just those chunks — not the entire Markdown file. A 2,000-line reference doc costs you one small section.

3. **Content-addressed caching.** Verdicts are a pure function of `(model, structural-diff, doc-chunk)` and are cached in `.driftlock/cache.json`. Identical checks — across commit amends, rebases, and CI re-runs — never hit the LLM again. On by default. See [Caching](./caching.md).

4. **A cheap model is enough.** The verdict task is "does this doc still describe this signature?" — a small, well-defined judgment. Inexpensive models such as `deepseek/deepseek-chat` handle it well. You do not need a frontier model.

### Practical guidance

- Keep `temperature = 0.0`: cheaper to reason about, and identical inputs produce identical cacheable outputs.
- Leave `include_full_diff = false` unless you observe the model misjudging drift without body context; turning it on multiplies token usage.
- Scope your `[[doc_mapping]]` tightly (see [Ignoring symbols](./ignoring.md)) so unrelated file changes never trigger a call at all.
- Use `DRIFTLOCK_DEBUG=1` to print token usage per request to stderr while you tune. See [Troubleshooting](./troubleshooting.md).

See also: [Configuration → `[llm]`](./configuration.md) · [Caching](./caching.md).
