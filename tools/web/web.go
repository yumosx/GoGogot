package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"gogogot/tools"
	"strings"
	"time"
)

type braveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func WebSearchTool(braveAPIKey string) tools.Tool {
	return tools.Tool{
		Name:        "web_search",
		Description: "Search the web for information using Brave Search. Returns top 5 results with title, URL, and description.",
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		Required: []string{"query"},
		Handler: func(ctx context.Context, input map[string]any) tools.Result {
			return webSearch(ctx, input, braveAPIKey)
		},
	}
}

func webSearch(ctx context.Context, input map[string]any, apiKey string) tools.Result {
	query, _ := input["query"].(string)
	if query == "" {
		return tools.Result{Output: "query is required", IsErr: true}
	}

	if apiKey == "" {
		slog.Warn("web_search: BRAVE_API_KEY not set")
		return tools.Result{Output: "BRAVE_API_KEY not set — web search disabled", IsErr: true}
	}

	slog.Debug("web_search", "query", query)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=5", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("request error: %v", err), IsErr: true}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Debug("web_search http error", "query", query, "error", err)
		return tools.Result{Output: fmt.Sprintf("http error: %v", err), IsErr: true}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("read body error: %v", err), IsErr: true}
	}

	slog.Debug("web_search response", "query", query, "status", resp.StatusCode, "body_len", len(body), "elapsed", time.Since(start))

	if resp.StatusCode != http.StatusOK {
		return tools.Result{Output: fmt.Sprintf("brave API %d: %s", resp.StatusCode, string(body)), IsErr: true}
	}

	var br braveResponse
	if err := json.Unmarshal(body, &br); err != nil {
		return tools.Result{Output: fmt.Sprintf("json decode error: %v", err), IsErr: true}
	}

	slog.Debug("web_search results", "query", query, "count", len(br.Web.Results))

	var sb strings.Builder
	for i, r := range br.Web.Results {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}
	if sb.Len() == 0 {
		return tools.Result{Output: "no results found"}
	}
	return tools.Result{Output: sb.String()}
}
