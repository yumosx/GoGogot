package episode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gogogot/internal/llm"
	"gogogot/internal/llm/types"
	"gogogot/internal/tools/store"

	"github.com/rs/zerolog/log"
)

type summaryResult struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// Close summarizes the episode via the LLM, then marks it closed and saves.
func (m *Manager) Close(ctx context.Context, ep *store.Episode) error {
	messages, err := ep.TextMessages()
	if err != nil || len(messages) == 0 {
		ep.Close()
		return ep.Save()
	}

	var transcript strings.Builder
	for _, msg := range messages {
		fmt.Fprintf(&transcript, "[%s]: %s\n", msg.Role, msg.Content)
	}

	prompt := "Summarize this conversation episode. Return ONLY valid JSON:\n" +
		`{"title": "short title", "summary": "2-3 sentence summary", "tags": ["tag1", "tag2"]}` +
		"\n\nPreserve: key decisions, outcomes, important facts, action items.\n\n---\n\n" +
		transcript.String()

	resp, err := m.llm.Call(ctx, []types.Message{
		types.NewUserMessage(types.TextBlock(prompt)),
	}, llm.CallOptions{
		System:  "You summarize conversations into structured JSON. Be concise and accurate.",
		NoTools: true,
	})

	if err != nil {
		log.Error().Err(err).Str("episode", ep.ID).Msg("episode: summarization failed")
		if len(messages) > 0 {
			ep.Title = store.TruncTitle(messages[0].Content)
		}
		ep.Summary = "(summarization failed)"
	} else {
		text := types.ExtractText(resp.Content)
		result := parseSummaryJSON(text)
		if result.Title != "" {
			ep.Title = result.Title
		} else if ep.Title == "" && len(messages) > 0 {
			ep.Title = store.TruncTitle(messages[0].Content)
		}
		ep.Summary = result.Summary
		ep.Tags = result.Tags
	}

	ep.Close()
	return ep.Save()
}

func parseSummaryJSON(text string) summaryResult {
	var result summaryResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start >= 0 && end > start {
			_ = json.Unmarshal([]byte(text[start:end+1]), &result)
		}
	}
	return result
}
