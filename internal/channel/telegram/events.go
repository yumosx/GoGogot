package telegram

import (
	"context"
	"gogogot/internal/core/transport"
)

func buildToolStatus(d transport.ToolStartData, plan []transport.PlanTask) transport.AgentStatus {
	label := toolLabel[d.Name]
	if label == "" {
		label = d.Name
	}
	if d.Detail != "" {
		label = label + ": " + d.Detail
	}

	phase := transport.PhaseTool
	if d.Name == "task_plan" {
		phase = transport.PhasePlanning
	}

	return transport.AgentStatus{Phase: phase, Tool: d.Name, Detail: label, Plan: plan}
}

func formatMessageWithLevel(text string, level transport.MessageLevel) string {
	switch level {
	case transport.LevelSuccess:
		return "✅ " + text
	case transport.LevelWarning:
		return "⚠️ " + text
	default:
		return "💡 " + text
	}
}

func (r *replier) ConsumeEvents(ctx context.Context, events <-chan transport.Event, replyInbox <-chan string) string {
	_ = r.SendTyping(ctx)
	statusID := r.sendStatus(ctx, transport.AgentStatus{Phase: transport.PhaseThinking})

	var (
		finalText   string
		currentPlan []transport.PlanTask
	)

	updateStatus := func(s transport.AgentStatus) {
		if statusID != 0 {
			r.updateStatus(ctx, statusID, s)
		}
	}

	restoreStatus := func() int {
		if statusID == 0 {
			return 0
		}
		return r.sendStatus(ctx, transport.AgentStatus{
			Phase: transport.PhaseWorking,
			Plan:  currentPlan,
		})
	}

	for ev := range events {
		switch ev.Kind {
		case transport.LLMStart:
			updateStatus(transport.AgentStatus{Phase: transport.PhaseThinking, Plan: currentPlan})
			_ = r.SendTyping(ctx)

		case transport.LLMStream:
			if d, ok := ev.Data.(transport.LLMStreamData); ok {
				finalText = d.Text
			}

		case transport.ToolStart:
			d, _ := ev.Data.(transport.ToolStartData)
			updateStatus(buildToolStatus(d, currentPlan))
			_ = r.SendTyping(ctx)

		case transport.Progress:
			d, _ := ev.Data.(transport.ProgressData)
			if d.Tasks != nil {
				currentPlan = d.Tasks
			}
			status := transport.AgentStatus{
				Phase:   transport.PhaseWorking,
				Plan:    currentPlan,
				Detail:  d.Status,
				Percent: d.Percent,
			}
			updateStatus(status)

		case transport.Message:
			d, _ := ev.Data.(transport.MessageData)
			text := formatMessageWithLevel(d.Text, d.Level)
			updateStatus(transport.AgentStatus{Phase: transport.PhaseMessage, Detail: text, Plan: currentPlan})

		case transport.Ask:
			d, _ := ev.Data.(transport.AskData)
			if statusID != 0 {
				r.deleteStatus(ctx, statusID)
			}
			_ = r.SendAsk(ctx, d.Prompt, d.Kind, d.Options)

			if replyInbox != nil {
				select {
				case resp := <-replyInbox:
					if d.ReplyCh != nil {
						d.ReplyCh <- resp
					}
				case <-ctx.Done():
					if d.ReplyCh != nil {
						close(d.ReplyCh)
					}
					return ""
				}
			} else {
				if d.ReplyCh != nil {
					d.ReplyCh <- "(no interactive input available)"
				}
			}
			statusID = restoreStatus()

		case transport.Error:
			if ctx.Err() != nil {
				return ""
			}
			d, _ := ev.Data.(transport.ErrorData)
			if statusID != 0 {
				r.deleteStatus(ctx, statusID)
			}
			_ = r.SendText(ctx, "Error: "+d.Error)
			return ""

		case transport.Done:
			if ctx.Err() != nil {
				r.deleteStatus(context.Background(), statusID)
				return ""
			}
			if finalText != "" {
				r.editToFinal(context.Background(), statusID, finalText)
			} else {
				r.deleteStatus(context.Background(), statusID)
			}
			return ""
		}
	}
	return ""
}
