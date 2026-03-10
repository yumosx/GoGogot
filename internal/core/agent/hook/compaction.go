package hook

import (
	"context"
	"fmt"
	"gogogot/internal/llm/types"
	"gogogot/internal/tools/store"
	"time"

	"github.com/rs/zerolog/log"
)

// Summarizer produces a concise summary from a conversation transcript.
type Summarizer func(ctx context.Context, prompt string) (string, error)

// CompactionBeforeIteration returns a BeforeIterationFunc that compacts
// conversation messages when they exceed the context window threshold.
func CompactionBeforeIteration(
	cfg store.CompactionConfig,
	summarize Summarizer,
) BeforeIterationFunc {
	return func(ctx context.Context, ic *IterationContext) {
		if ic.ContextWindow <= 0 || ic.Conversation == nil {
			return
		}

		msgs := ic.Conversation.Messages()
		estimated := store.EstimateTokens(msgs)
		if !cfg.ShouldCompact(estimated, ic.ContextWindow) {
			return
		}

		log.Info().
			Int("estimated_tokens", estimated).
			Int("context_window", ic.ContextWindow).
			Float64("threshold", cfg.Threshold).
			Msg("compaction triggered")

		n := len(msgs)
		if n <= cfg.PreserveRecent {
			return
		}

		cutoff := n - cfg.PreserveRecent
		old := msgs[:cutoff]
		recent := msgs[cutoff:]

		transcript := store.RenderTranscript(old)
		prompt := cfg.SummaryPrompt + "\n\n---\n\n" + transcript

		summary, err := summarize(ctx, prompt)
		if err != nil {
			log.Error().Err(err).Msg("compaction summarize failed")
			return
		}

		compacted := make([]store.Turn, 0, 1+len(recent))
		compacted = append(compacted, store.Turn{
			Role:      string(types.RoleUser),
			Content:   []types.ContentBlock{types.TextBlock(fmt.Sprintf("[Context Summary]\n%s", summary))},
			Timestamp: time.Now(),
			Metadata:  map[string]any{"compacted": true, "original_messages": len(old)},
		})
		compacted = append(compacted, recent...)

		if err := ic.Conversation.ReplaceMessages(compacted); err != nil {
			log.Error().Err(err).Msg("compaction rewrite failed")
			return
		}

		after := store.EstimateTokens(compacted)
		log.Info().Int("before", estimated).Int("after", after).Msg("compaction done")
	}
}
