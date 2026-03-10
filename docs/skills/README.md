## Voxray Skills Map

This `docs/skills` set is an **onboarding map** for new contributors: what you need to know, where it lives in the code, and how hard it is to ramp up.

---

### Files & Reading Order

- **1. Core Stack** — `docs/skills/core-stack.md`
  - Languages, runtimes, and major non‑AI libraries (Go, WebSocket/WebRTC, audio/Opus, Prometheus, Swagger).
  - Read this first if you are new to Voxray or coming from a non‑Go background.

- **2. AI/ML Stack** — `docs/skills/ai-ml-stack.md`
  - All LLM/STT/TTS providers, how they are wired, and how prompts, tools, and context frames work.
  - Essential for anyone touching providers, prompt flows, or tool‑calling.

- **3. Infrastructure** — `docs/skills/infrastructure.md`
  - Deployment, Docker, Redis session store, Postgres/MySQL transcripts, S3 recordings, telephony/Daily, networking, and secrets.
  - Start here if you are focused on SRE/infra or running Voxray in production.

- **4. DevOps & Tooling** — `docs/skills/devops-tooling.md`
  - CI, Makefile, scripts, testing layout (unit, integration, stress), evals, linting, and observability wiring.
  - Useful for anyone touching tests, pipelines, or build/CI configuration.

- **5. Key Concepts** — `docs/skills/key-concepts.md`
  - Architectural patterns, frames as a DSL, processors/observers, extensions (IVR, voicemail), MCP tools, and runtime flow (with a Mermaid diagram).
  - Read after you have a basic feel for the code; this is where the “mental model” of Voxray lives.

---

### Onboarding Complexity Ratings

| Section | File | Complexity | Why |
|--------|------|------------|-----|
| **1. Core Stack** | `core-stack.md` | **Medium** | Mostly standard Go and networking patterns, but requires solid understanding of concurrency, channels, and audio/metrics libraries to work safely in the core server. |
| **2. AI/ML Stack** | `ai-ml-stack.md` | **High** | Many providers and streaming patterns, plus prompt/context/tool abstractions (frames, summarizers, MCP) that interact in subtle ways. |
| **3. Infrastructure** | `infrastructure.md` | **Medium** | Concepts (Docker, Redis, Postgres/MySQL, S3, TLS, CORS) are common, but weaving them into Voxray’s topology (runner modes, telephony, Daily) takes some study. |
| **4. DevOps & Tooling** | `devops-tooling.md` | **Medium** | Tooling is conventional (Go, Make, GitHub Actions, Prometheus), but there are several layers of tests (stress, evals, frontend) and scripts to understand. |
| **5. Key Concepts** | `key-concepts.md` | **High** | Requires synthesizing frames, processors, observers, extensions, and tools into a single mental model; most architectural changes depend on this understanding. |

---

### Role‑Based Onboarding Suggestions

- **Pipeline / core server engineer**
  - Read in order: **Core Stack → Key Concepts → AI/ML Stack → DevOps & Tooling → Infrastructure**.
  - Early tasks: add a simple processor, hook up a new metric, or extend an existing frame type.

- **Provider / AI integrations engineer**
  - Read in order: **AI/ML Stack → Core Stack → Key Concepts → DevOps & Tooling**.
  - Early tasks: add or modify a provider adapter, update `config` examples, and add eval scenarios.

- **Infra / SRE / platform engineer**
  - Read in order: **Infrastructure → DevOps & Tooling → Core Stack → Key Concepts**.
  - Early tasks: stand up a production‑like deployment (Docker or Kubernetes), configure metrics scraping, wire Redis/session store and transcripts DB, and configure session caps and observe `active_sessions` / `sessions_rejected_total`. 

