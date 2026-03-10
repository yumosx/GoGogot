package agent

import (
	"gogogot/internal/core/agent/hook"
	"gogogot/internal/llm/types"
	"gogogot/internal/tools/store"
	"time"
)

type parsedResponse struct {
	assistantBlocks []types.ContentBlock
	toolCalls       []types.ContentBlock
	textContent     string
}

func appendUserMessage(conv hook.Conversation, userBlocks []types.ContentBlock) {
	conv.AppendMessage(store.Turn{
		Role:      string(types.RoleUser),
		Content:   userBlocks,
		Timestamp: time.Now(),
	})
}

func buildLLMMessages(conv hook.Conversation) []types.Message {
	turns := conv.Messages()
	msgs := make([]types.Message, 0, len(turns))
	for _, t := range turns {
		role := types.RoleUser
		if t.Role == string(types.RoleAssistant) {
			role = types.RoleAssistant
		}
		msgs = append(msgs, types.Message{Role: role, Content: t.Content})
	}
	return msgs
}

func parseResponseBlocks(content []types.ContentBlock) parsedResponse {
	var p parsedResponse
	for _, block := range content {
		switch block.Type {
		case "tool_use":
			p.toolCalls = append(p.toolCalls, block)
			p.assistantBlocks = append(p.assistantBlocks, block)
		case "text":
			p.textContent += block.Text
			p.assistantBlocks = append(p.assistantBlocks, block)
		}
	}
	return p
}
