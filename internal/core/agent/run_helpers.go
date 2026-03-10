package agent

import (
	"context"
	"gogogot/internal/core/agent/hook"
	"gogogot/internal/llm"
	"gogogot/internal/llm/types"
)

func (a *Agent) summarize(ctx context.Context, prompt string) (string, error) {
	msgs := []types.Message{
		types.NewUserMessage(types.TextBlock(prompt)),
	}
	resp, err := a.client.Call(ctx, msgs, llm.CallOptions{
		System:  a.config.Compaction.SummaryPrompt,
		NoTools: true,
	})
	if err != nil {
		return "", err
	}
	return types.ExtractText(resp.Content), nil
}

func (a *Agent) runBeforeHooks(ctx context.Context, ic *hook.IterationContext) {
	for _, h := range a.beforeHooks {
		h(ctx, ic)
	}
}

func (a *Agent) runAfterHooks(ctx context.Context, ic *hook.IterationContext, result *hook.IterationResult) {
	for _, h := range a.afterHooks {
		h(ctx, ic, result)
	}
}
