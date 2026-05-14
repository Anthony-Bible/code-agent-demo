# Anthropic Go SDK vs Firebase Genkit Go — Compatibility Research

**Date:** 2026-04-02  
**Anthropic SDK:** `github.com/anthropics/anthropic-sdk-go`  
**Genkit Go:** `github.com/firebase/genkit/go` (Anthropic plugin at `go/v1.6.0`)  
**Genkit Anthropic plugin source:** [`go/plugins/internal/anthropic/anthropic.go`](https://github.com/genkit-ai/genkit/blob/go/v1.6.0/go/plugins/internal/anthropic/anthropic.go)

---

## Overview

Firebase Genkit Go is a framework that wraps AI providers behind a unified `ai.Model`
interface. Its Anthropic plugin uses the Anthropic Go SDK internally — it is a thin
translation layer, not a reimplementation. This has an important consequence: almost
every Anthropic SDK feature remains accessible, either through Genkit's high-level API
or via a raw `anthropic.MessageNewParams` escape hatch.

---

## Core message generation

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Non-streaming generation | `client.Messages.New(ctx, params)` | `ai.Generate(ctx, registry, opts...)` |
| Streaming generation | `client.Messages.NewStreaming(ctx, params)` | `ai.GenerateStream(ctx, registry, opts...)` |
| System prompt | `params.System []TextBlockParam` | `ai.WithSystem(text)` or via messages |
| Max tokens | `params.MaxTokens int64` | `ai.WithConfig(anthropic.MessageNewParams{MaxTokens: ...})` |
| Stop sequences | `params.StopSequences []string` | Via raw config escape hatch |
| Temperature / top-p / top-k | `params.Temperature`, etc. | Via raw config escape hatch |
| Model selection | Typed constant or string (`anthropic.Model(s)`) | Registered string (`"anthropic/claude-..."`) |

---

## Streaming

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Text deltas | `ContentBlockDeltaEvent` → `TextDelta.Text` | `*ai.Part` with `p.IsText()` |
| Thinking deltas | `ContentBlockDeltaEvent` → `ThinkingDelta.Thinking` | `*ai.Part` with `p.IsReasoning()` — emitted as `thinking_delta` |
| Tool use blocks | `ContentBlockStopEvent` → `ToolUseBlock` | `*ai.Part` with `p.IsToolRequest()` |
| Accumulating full response | `message.Accumulate(event)` | Handled internally; final `ModelResponse` returned |
| Streaming callback style | Server-sent events via `stream.Next()` loop | Single `cb func(ctx, *ModelResponseChunk) error` per chunk |
| Error handling | `stream.Err()` after loop | `stream.Err()` propagated through iterator |

The Genkit plugin uses a **single callback** for all content types. Callers demux by
inspecting each `*ai.Part` in the chunk's `Content` slice.

---

## Tool use / function calling

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Define tools | `anthropic.ToolParam{Name, Description, InputSchema}` | `ai.DefineTool[In, Out]()` with typed generics |
| Tool use in request | `params.Tools []ToolUnionParam` | `ai.WithTools(tools...)` |
| Tool call in response | `content.AsAny().(anthropic.ToolUseBlock)` | `part.ToolRequest` on `p.IsToolRequest()` parts |
| Tool results | `anthropic.NewToolResultBlock(id, result, isError)` | `p.IsToolResponse()` parts |
| Manual tool loop control | Full control — caller drives the loop | `ai.WithReturnToolRequests(true)` for manual control |
| Automatic tool loop | Not built-in | Genkit can run the tool loop automatically |
| Human-in-the-loop interrupts | Not built-in | `InterruptWith()` / `RestartWith()` |
| Strict schemas | Opt-in | **Enforced by default** (`additionalProperties: false` recursively) |
| Tool name validation | No enforced regex | Must match `^[a-zA-Z0-9_-]{1,64}$` |

> **Note:** Genkit's strict schema enforcement and tool name regex are **additive
> constraints** not present in the raw SDK. Tools that work with the raw SDK may need
> name adjustments for Genkit.

---

## Extended thinking (reasoning)

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Enable thinking | `ThinkingConfigParamOfEnabled(budgetTokens)` | Pass `anthropic.MessageNewParams{Thinking: ...}` as `.Config` |
| Disable thinking | `ThinkingConfigDisabledParam{}` | Omit from config |
| `budget_tokens` control | Direct field | Via raw config escape hatch |
| Receive thinking blocks | `content.AsAny().(anthropic.ThinkingBlock)` | `ai.NewReasoningPart(text, signature)` |
| Thinking signatures (round-trip) | `content.Signature string` | `[]byte` stored in `ai.Part` metadata |
| Redacted thinking blocks | `content.Type == "redacted_thinking"` | Not explicitly handled — falls through as unknown part |
| Streaming thinking deltas | `ThinkingDelta.Thinking` | `event.Delta.Type == "thinking_delta"` → `ai.NewReasoningPart` |

The escape hatch in `configFromRequest` makes budget tokens fully controllable:

```go
// Genkit: pass raw SDK struct as config to unlock any Anthropic-specific param
ai.WithConfig(anthropic.MessageNewParams{
    MaxTokens: 16000,
    Thinking:  anthropic.ThinkingConfigParamOfEnabled(10000), // budget_tokens
})
```

---

## Token usage

| Field | Anthropic SDK | Genkit |
|---|---|---|
| Input tokens | `response.Usage.InputTokens int64` | `r.Usage.InputTokens int` |
| Output tokens | `response.Usage.OutputTokens int64` | `r.Usage.OutputTokens int` |
| Cache read tokens | `response.Usage.CacheReadInputTokens int64` | `r.Usage.CachedContentTokens int` |
| Cache write tokens | `response.Usage.CacheCreationInputTokens int64` | Not mapped |
| Type | `int64` | `int` — potential truncation on very large values |

---

## Prompt caching

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Cache breakpoints | `anthropic.CacheControlEphemeralParam` on content blocks | Not exposed in high-level API |
| Cache read token tracking | `Usage.CacheReadInputTokens` | `r.Usage.CachedContentTokens` |
| Cache write token tracking | `Usage.CacheCreationInputTokens` | Not mapped |

Prompt caching requires the raw SDK or the escape hatch to set cache control params.

---

## Vision / media

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Base64 image blocks | `anthropic.NewImageBlockBase64(mediaType, data)` | `ai.NewMediaPart(url)` or `ai.NewDataPart(data)` → translated in `toAnthropicParts` |
| URL image blocks | `anthropic.NewImageBlockURL(url)` | Via `p.IsMedia()` with URL data URI |
| PDF documents | `anthropic.NewDocumentBlockBase64(...)` | Not mapped in `toAnthropicParts` — falls to `unknown part type` error |
| Multiple media in one message | Supported | Supported |

> **PDF documents are a gap.** The Genkit plugin's `toAnthropicParts` handles text,
> media, data, tool request, tool response, and reasoning — but not PDF document blocks.

---

## Model management

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Model as string | `anthropic.Model(anyString)` | Must be a registered model name |
| Model discovery | `client.Models.ListAutoPaging()` | Plugin fetches available models at init |
| Custom base URL | `ANTHROPIC_BASE_URL` env var or `WithBaseURL` option | `BaseURL` field on the plugin struct |
| Custom model strings (e.g. `hf:*`) | Passed through as-is | **Not supported** — registration system requires known names |
| Per-request model override | Not applicable (set on client) | `ai.WithModelName(name)` |

The model registration system is the main structural difference. Genkit requires
models to be registered at startup; you cannot pass an arbitrary model string at
request time without first registering it.

---

## Batch API

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Create batch | `client.Beta.Messages.Batches.New(...)` | Not supported |
| Poll batch results | `client.Beta.Messages.Batches.ResultsStreaming(...)` | Not supported |

The Batch API is entirely absent from Genkit. If bulk processing is needed, use the
Anthropic SDK directly.

---

## Structured / constrained output

| Capability | Anthropic SDK | Genkit |
|---|---|---|
| Native JSON schema output | `params.OutputConfig` (`claude-3-7` and later) | `i.Output.Format == "json"` → `req.OutputConfig` |
| Schema enforcement | Manual | Genkit enforces `additionalProperties: false` recursively |
| Typed output | Manual unmarshalling | `ai.Generate[MyStruct]()` with generic type parameter |

---

## Raw response access

Genkit exposes `r.Raw` which holds the original `anthropic.Message.JSON` field. This
means any response data not explicitly mapped by Genkit (e.g. `ThoughtSignature` on
tool use blocks, Gemini bifrost metadata) can be read from `r.Raw`.

---

## What Genkit adds beyond the raw SDK

- **Provider abstraction** — same `ai.Generate()` call works for Gemini, Vertex AI,
  Ollama, and OpenAI-compatible APIs
- **Typed tool definitions** — `genkit.DefineTool[In, Out]()` with compile-time type safety
- **Automatic tool-calling loop** — Genkit can run the tool loop without caller boilerplate
- **Human-in-the-loop** — `InterruptWith()` / `RestartWith()` for tool interrupts
- **Flows & Prompts** — reusable, versioned prompt templates with YAML frontmatter
- **Middleware** — global request/response interception
- **Developer UI** — local trace inspection and model comparison (dev mode)
- **Observable execution** — built-in tracing for all generate calls

---

## Summary: when to use each

| Use case | Recommendation |
|---|---|
| Claude-only app, need full API surface | Anthropic SDK directly |
| Multi-provider app (Claude + Gemini + Ollama) | Genkit |
| Extended thinking with budget control | Either — Genkit supports via escape hatch |
| Batch API / bulk processing | Anthropic SDK only |
| PDF document input | Anthropic SDK only (Genkit gap) |
| Prompt caching with cache breakpoints | Anthropic SDK (Genkit doesn't expose control) |
| Custom / non-Anthropic model strings (e.g. `hf:*`) | Anthropic SDK, or custom Genkit plugin |
| Typed, safe tool definitions | Genkit preferred |
| Human-in-the-loop workflows | Genkit preferred |
