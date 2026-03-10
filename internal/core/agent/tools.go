package agent

import (
	"context"
	llmtypes "gogogot/internal/llm/types"
	"gogogot/internal/tools/types"
	"net/url"
	"path/filepath"
)

func (a *Agent) localToolDefs() []llmtypes.ToolDef {
	defs := make([]llmtypes.ToolDef, 0, len(a.localTools))
	for _, t := range a.localTools {
		defs = append(defs, llmtypes.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
			Required:    t.Required,
		})
	}
	return defs
}

func (a *Agent) executeLocal(ctx context.Context, name string, input map[string]any) (types.Result, bool) {
	t, ok := a.localTools[name]
	if !ok {
		return types.Result{}, false
	}
	return t.Handler(ctx, input), true
}

func (a *Agent) executeTool(ctx context.Context, name string, input map[string]any) types.Result {
	if result, handled := a.executeLocal(ctx, name, input); handled {
		return result
	}
	return a.registry.Execute(ctx, name, input)
}

const maxDetailLen = 60

func extractToolDetail(name string, input map[string]any) string {
	var detail string
	switch name {
	case "bash":
		detail, _ = input["command"].(string)
	case "edit_file", "read_file", "write_file":
		if p, ok := input["path"].(string); ok {
			detail = filepath.Base(p)
		}
	case "web_search":
		detail, _ = input["query"].(string)
	case "web_fetch":
		if raw, ok := input["url"].(string); ok {
			if u, err := url.Parse(raw); err == nil {
				detail = u.Host
			}
		}
	}
	if len(detail) > maxDetailLen {
		detail = detail[:maxDetailLen] + "..."
	}
	return detail
}
