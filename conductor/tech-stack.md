# Project Tech Stack

This document outlines the core technology stack and architectural principles of the AI-powered SRE assistant.

## 1. Core Language & Runtime
- **Language:** [Go (1.24+)](https://go.dev/)
- **Build System:** Standard Go toolchain with `go mod` for dependency management.

## 2. CLI & Application Frameworks
- **CLI Engine:** [Cobra](https://github.com/spf13/cobra) — Provides a robust structure for terminal-based commands and flags.
- **Configuration:** [Viper](https://github.com/spf13/viper) — Manages environment variables, flags, and configuration files (YAML, TOML).
- **Interactive Input:** [Readline](https://github.com/chzyer/readline) — For the chat interface history and input handling.

## 3. AI & Natural Language Processing
- **LLM Provider:** [Anthropic SDK for Go](https://github.com/anthropics/anthropic-sdk-go) — Direct integration with Claude (3.5 Sonnet or newer).
- **Context Management:** Custom "Auto-Compaction" system for handling long-running conversation history.
- **Reasoning:** Native support for Claude's "Extended Thinking" mode for complex troubleshooting.
- **Correlation Engine:** Domain-driven `RCAService` that uses LLM reasoning to synthesize investigation findings into structured root causes.

## 4. SRE & Infrastructure Integration
- **Alert Ingestion:** Built-in HTTP webhook server for receiving alerts.
- **Supported Sources:**
  - [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/)
  - [GCP Monitoring (Cloud Monitoring)](https://cloud.google.com/monitoring)
- **Investigation Store:** Local file-based persistence for tracking alert investigations and findings.

## 5. Architectural Principles
- **Pattern:** [Hexagonal Architecture](https://en.wikipedia.org/wiki/Hexagonal_architecture_(software)) (Ports & Adapters).
- **Benefits:** Ensures the core domain (Conversation, Alert, Investigation) is isolated from infrastructure concerns (AI provider, File system, Webhooks).
- **Modularity:** High degree of testability through clear port definitions and adapter implementations.

## 6. Testing & Quality Assurance
- **Testing Framework:** [Testify](https://github.com/stretchr/testify) for assertions and mock management.
- **Methodology:** Heavy emphasis on table-driven unit tests across domain and application layers.
- **Integration Testing:** End-to-end verification of the investigation pipeline, using reflection for dependency injection in complex use cases.
- **Coverage:** Targeted 80%+ test coverage for core business logic.

## 7. Key Third-Party Libraries
- `golang.org/x/net`: For HTTP utilities and webhook handling.
- `gopkg.in/yaml.v3`: For parsing SKILL.md and AGENT.md metadata.
- `github.com/tidwall/gjson`: For fast JSON parsing during tool and alert processing.