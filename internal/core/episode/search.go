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

const maxSummariesForSearch = 50

const searchSystem = `You search through conversation history summaries.
You receive a numbered list of past conversation summaries and a query.
Return ONLY a JSON array of the numbers that are relevant to the query.
Example: [1, 4, 7]
If nothing is relevant, return: []
No explanation, no extra text.`

// SearchRelevant uses the LLM to find past episodes semantically related to the query.
func (m *Manager) SearchRelevant(ctx context.Context, query string) ([]store.EpisodeInfo, error) {
	all, err := m.store.ListEpisodes()
	if err != nil {
		return nil, err
	}

	var closed []store.EpisodeInfo
	for _, ep := range all {
		if ep.Status == "closed" && ep.Summary != "" {
			closed = append(closed, ep)
		}
	}
	if len(closed) == 0 {
		return nil, nil
	}
	if len(closed) > maxSummariesForSearch {
		closed = closed[:maxSummariesForSearch]
	}

	var catalog strings.Builder
	for i, ep := range closed {
		title := ep.Title
		if title == "" {
			title = "Untitled"
		}
		date := ep.StartedAt.Format("02 Jan 2006")
		fmt.Fprintf(&catalog, "%d. [%s] (%s): %s", i+1, title, date, ep.Summary)
		if len(ep.Tags) > 0 {
			fmt.Fprintf(&catalog, " [tags: %s]", strings.Join(ep.Tags, ", "))
		}
		catalog.WriteByte('\n')
	}

	prompt := fmt.Sprintf("Past conversations:\n%s\n---\nQuery: %s", catalog.String(), query)

	resp, err := m.llm.Call(ctx, []types.Message{
		types.NewUserMessage(types.TextBlock(prompt)),
	}, llm.CallOptions{
		System:  searchSystem,
		NoTools: true,
	})
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("episode: semantic search LLM call failed")
		return nil, fmt.Errorf("search LLM call: %w", err)
	}

	indices := parseSearchResponse(types.ExtractText(resp.Content))

	var matches []store.EpisodeInfo
	for _, idx := range indices {
		if idx >= 1 && idx <= len(closed) {
			matches = append(matches, closed[idx-1])
		}
	}

	const maxResults = 5
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	log.Debug().Str("query", query).Int("matches", len(matches)).Msg("episode: semantic search")
	return matches, nil
}

func parseSearchResponse(text string) []int {
	text = strings.TrimSpace(text)

	var indices []int
	if err := json.Unmarshal([]byte(text), &indices); err != nil {
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start >= 0 && end > start {
			_ = json.Unmarshal([]byte(text[start:end+1]), &indices)
		}
	}
	return indices
}
