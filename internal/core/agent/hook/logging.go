package hook

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func LoggingBeforeIteration() BeforeIterationFunc {
	return func(_ context.Context, ic *IterationContext) {
		evt := log.Debug().
			Int("iteration", ic.Iteration).
			Str("model", ic.Model).
			Int("message_count", len(ic.Messages))

		if ic.System != "" {
			evt.Str("system_prompt", truncateStr(ic.System, 300))
		}

		arr := zerolog.Arr()
		for _, msg := range ic.Messages {
			arr.Dict(zerolog.Dict().
				Str("role", string(msg.Role)).
				Str("types", blockTypeSummary(msg.Content)).
				Str("preview", truncateStr(textFromBlocks(msg.Content), 150)))
		}
		evt.Array("messages", arr)

		evt.Msg("iteration start")
	}
}

func LoggingAfterIteration() AfterIterationFunc {
	return func(_ context.Context, ic *IterationContext, result *IterationResult) {
		resp := result.Response
		evt := log.Info().
			Int("iteration", ic.Iteration).
			Str("model", ic.Model).
			Dur("llm_elapsed", result.LLMDuration).
			Int("input_tokens", resp.InputTokens).
			Int("output_tokens", resp.OutputTokens).
			Str("stop_reason", resp.StopReason)

		text := textFromBlocks(resp.Content)
		if text != "" {
			evt.Str("response_text", text)
		}

		if len(result.ToolCalls) > 0 {
			tools := zerolog.Arr()
			for _, tc := range result.ToolCalls {
				tools.Dict(zerolog.Dict().
					Str("name", tc.Name).
					Dur("duration", tc.Duration).
					Bool("is_err", tc.IsErr))
			}
			evt.Array("tool_calls", tools)
		}

		evt.Msg("iteration done")
	}
}
