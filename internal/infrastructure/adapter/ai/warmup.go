// Package ai — warmup helpers prime Anthropic's compiled-grammar cache by
// issuing a single tool-bearing request at startup. This amortizes the
// 30-100s first-call latency that strict-mode tools incur before the
// regional cache is warm.
package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// WarmCacheTimeout bounds the warm-up call. Anthropic's first-call grammar
// compilation has been observed at ~70-100s in our tests, so we give it
// ample headroom.
const WarmCacheTimeout = 3 * time.Minute

// WarmCache primes the AI provider's tool-schema cache by sending a minimal
// request that carries the full tool catalog. The response is discarded.
//
// Cache invalidation is sensitive to the JSON schema structure and the SET
// of tools in the request, per Anthropic's docs — so callers MUST pass the
// exact same tool catalog (in the exact same shape) that real requests will
// use. Tool order is normalized by ExecutorAdapter.ListTools, which sorts
// by name; pass tools through that path rather than building them ad-hoc.
//
// Returns nil on success, or a wrapping error if the provider call fails.
// The caller is expected to log+continue rather than fail startup: a cold
// cache is a latency penalty, not a correctness problem.
func WarmCache(
	ctx context.Context,
	p port.AIProvider,
	tools []port.ToolParam,
	log port.Logger,
) error {
	if p == nil {
		return errors.New("warmup: AI provider is nil")
	}
	log = port.SafeLogger(log)

	if len(tools) == 0 {
		log.Info("warmup: skipping — no tools registered")
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, WarmCacheTimeout)
	defer cancel()

	messages := []port.MessageParam{
		{
			Role:    entity.RoleUser,
			Content: "ping",
		},
	}

	// SystemPrompt set explicitly so getSystemPrompt skips the subagent
	// listing path (which would issue extra I/O during warmup).
	opts := port.AIRequestOptions{
		SystemPrompt: "Reply with a single word.",
	}

	start := time.Now()
	log.Info("warmup: priming AI provider grammar cache",
		"tool_count", len(tools),
		"timeout", WarmCacheTimeout,
	)

	if _, _, err := p.SendMessage(ctx, messages, tools, opts); err != nil {
		return fmt.Errorf("warmup: SendMessage failed after %s: %w", time.Since(start), err)
	}

	log.Info("warmup: grammar cache primed",
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// ToolParamsFromExecutor pulls the deterministic tool catalog from a
// ToolExecutor and converts it via port.ToolParamFromEntity so the schema
// fingerprint is identical to what real requests send.
func ToolParamsFromExecutor(exec port.ToolExecutor) ([]port.ToolParam, error) {
	if exec == nil {
		return nil, errors.New("warmup: ToolExecutor is nil")
	}
	tools, err := exec.ListTools()
	if err != nil {
		return nil, fmt.Errorf("warmup: ListTools failed: %w", err)
	}
	out := make([]port.ToolParam, len(tools))
	for i, t := range tools {
		out[i] = port.ToolParamFromEntity(t)
	}
	return out, nil
}
