package core

import (
	"context"
	"encoding/json"
	"fmt"
	"gogogot/internal/channel"
	"gogogot/internal/core/agent"
	event2 "gogogot/internal/core/agent/event"
	"gogogot/internal/core/agent/prompt"
	transport2 "gogogot/internal/core/transport"
	"gogogot/internal/infra/config"
	"gogogot/internal/infra/scheduler"
	llm2 "gogogot/internal/llm"
	"gogogot/internal/llm/types"
	"gogogot/internal/tools"
	store2 "gogogot/internal/tools/store"
	"gogogot/internal/tools/system"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const defaultEpisodeGap = 2 * time.Hour

type Engine struct {
	transport  *transport2.Transport
	llmClient  llm2.LLM
	agent      *agent.Agent
	store      *store2.Store
	scheduler  *scheduler.Scheduler
	registry   *tools.Registry
	episodeGap time.Duration

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func New(cfg *config.Config, ch channel.Channel) (*Engine, error) {

	st, err := store2.New(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	provider, err := resolveProvider(cfg)
	if err != nil {
		return nil, err
	}

	sched := scheduler.New(cfg.DataDir, nil, st.LoadTimezone())

	t := transport2.New(ch)

	extra := append(transport2.ChannelTools(),
		system.ScheduleTools(sched)...,
	)
	extra = append(extra, st.IdentityTools(sched.SetLocation)...)

	reg := tools.NewRegistry(st, cfg.BraveAPIKey, extra...)

	client := llm2.NewClient(*provider, reg.Definitions())

	transportName := ch.Name()
	modelLabel := provider.Label
	agentCfg := agent.Config{
		PromptLoader: func() prompt.PromptContext {
			skills, _ := store2.LoadSkills(st.SkillsDir())
			return prompt.PromptContext{
				TransportName: transportName,
				ModelLabel:    modelLabel,
				Soul:          st.ReadSoul(),
				User:          st.ReadUser(),
				SkillsBlock:   store2.FormatSkillsForPrompt(skills),
				Timezone:      st.LoadTimezone(),
			}
		},
		MaxTokens:  cfg.MaxTokens,
		Compaction: store2.DefaultCompactionConfig(),
	}

	eng := &Engine{
		transport:  t,
		llmClient:  client,
		store:      st,
		scheduler:  sched,
		registry:   reg,
		episodeGap: defaultEpisodeGap,
		cancels:    make(map[string]context.CancelFunc),
	}
	eng.agent = agent.New(client, agentCfg, reg)

	ownerChannelID := t.OwnerChannelID()
	sched.SetExecutor(func(ctx context.Context, taskID, command, skill string) (string, error) {
		return eng.RunScheduledTask(ctx, ownerChannelID, taskID, command, skill)
	})

	return eng, nil
}

func (e *Engine) Run(ctx context.Context) error {
	if err := e.scheduler.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer e.scheduler.Stop()

	return e.transport.Run(ctx, e.handleMessage)
}

func resolveProvider(cfg *config.Config) (*llm2.Provider, error) {
	if cfg.Provider == "" {
		return nil, fmt.Errorf("GOGOGOT_PROVIDER is required — set to 'anthropic', 'openai', or 'openrouter'")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("GOGOGOT_MODEL is required — use an exact model ID (e.g. claude-sonnet-4-6, gpt-4o) or an OpenRouter slug (vendor/model)")
	}
	return llm2.ResolveProvider(cfg.Model, cfg.Provider)
}

func (e *Engine) Transport() *transport2.Transport {
	return e.transport
}

func (e *Engine) handleMessage(ctx context.Context, msg channel.Message) {
	if msg.Command != nil {
		e.handleCommand(ctx, msg)
		return
	}

	channelID := msg.ChannelID

	e.mu.Lock()
	_, busy := e.cancels[channelID]
	e.mu.Unlock()

	if busy {
		_ = e.transport.SendText(ctx, channelID, "Still working on the previous task, please wait...")
		return
	}

	if msg.Text == "" && len(msg.Attachments) == 0 {
		return
	}

	log.Info().
		Str("channel", channelID).
		Int("text_len", len(msg.Text)).
		Int("attachments", len(msg.Attachments)).
		Msg("engine: incoming message")

	go e.runAgent(ctx, channelID, msg)
}

func (e *Engine) handleCommand(ctx context.Context, msg channel.Message) {
	cmd := msg.Command
	switch cmd.Name {
	case channel.CmdNewEpisode:
		cmd.Result.Error = e.resetEpisode(ctx, msg.ChannelID)
	case channel.CmdStop:
		e.stopAgent(ctx, msg.ChannelID)
	case channel.CmdHistory:
		episodes, err := e.store.ListEpisodes()
		if err != nil {
			cmd.Result.Error = err
		} else {
			cmd.Result.Data = map[string]string{"text": transport2.FormatHistory(episodes)}
		}
	case channel.CmdMemory:
		files, err := e.store.ListMemory()
		if err != nil {
			cmd.Result.Error = err
		} else {
			cmd.Result.Data = map[string]string{"text": transport2.FormatMemory(files)}
		}
	}
}

func (e *Engine) runAgent(ctx context.Context, channelID string, msg channel.Message) {
	agentCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	e.mu.Lock()
	e.cancels[channelID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, channelID)
		e.mu.Unlock()
	}()

	ep, err := e.loadEpisode(agentCtx, channelID)
	if err != nil {
		log.Error().Err(err).Msg("engine: failed to load episode")
		_ = e.transport.SendText(ctx, channelID, "Error: "+err.Error())
		return
	}

	e.transport.NotifyTyping(ctx, channelID)
	statusID := e.transport.SendInitialStatus(ctx, channelID)

	agentCtx = channel.WithChannel(agentCtx, e.transport.Channel(), channelID)

	blocks, cleanup := transport2.ProcessAttachments(ep.ID, msg.Text, msg.Attachments)
	defer cleanup()

	bus, recv := event2.NewBus(64)
	go func() {
		defer bus.Close()
		if err := e.agent.Run(agentCtx, ep, blocks, bus); err != nil {
			log.Error().Err(err).Str("channel", channelID).Msg("engine: agent run failed")
		}
	}()

	finalText := e.transport.ConsumeEvents(agentCtx, channelID, recv, statusID)
	if finalText != "" {
		_ = e.transport.SendText(ctx, channelID, finalText)
	}
}

func (e *Engine) stopAgent(ctx context.Context, channelID string) {
	e.mu.Lock()
	cancel, running := e.cancels[channelID]
	e.mu.Unlock()

	if !running {
		_ = e.transport.SendText(ctx, channelID, "Nothing to cancel.")
		return
	}

	cancel()
	_ = e.transport.SendText(ctx, channelID, "⏹ Stopping...")
}

// RunScheduledTask executes a scheduled task in the active episode for the
// given channel. It runs synchronously and returns the agent's text output.
// If the agent is already busy on this channel, it returns an error so the
// scheduler can apply backoff and retry later.
func (e *Engine) RunScheduledTask(ctx context.Context, channelID, taskID, command, skill string) (string, error) {
	e.mu.Lock()
	_, busy := e.cancels[channelID]
	e.mu.Unlock()
	if busy {
		return "", fmt.Errorf("agent busy on channel %s, will retry", channelID)
	}

	agentCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	agentCtx = channel.WithChannel(agentCtx, e.transport.Channel(), channelID)

	e.mu.Lock()
	e.cancels[channelID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, channelID)
		e.mu.Unlock()
	}()

	ep, err := e.loadEpisode(agentCtx, channelID)
	if err != nil {
		return "", fmt.Errorf("load episode: %w", err)
	}

	promptText := buildScheduledPrompt(taskID, command, skill)
	blocks := []types.ContentBlock{types.TextBlock(promptText)}

	bus, recv := event2.NewBus(64)
	var runErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer bus.Close()
		runErr = e.agent.Run(agentCtx, ep, blocks, bus)
	}()

	var finalText string
	for ev := range recv {
		if ev.Kind == event2.LLMStream {
			if d, ok := ev.Data.(event2.LLMStreamData); ok {
				finalText = d.Text
			}
		}
	}
	<-done

	if runErr != nil {
		return "", runErr
	}

	if finalText != "" {
		_ = e.transport.SendText(ctx, channelID, finalText)
	}

	return finalText, nil
}

func (e *Engine) loadEpisode(ctx context.Context, channelID string) (*store2.Episode, error) {
	ep, err := e.store.LoadOrCreateActiveEpisode(channelID)
	if err != nil {
		return nil, err
	}

	if ep.HasMessages() && time.Since(ep.UpdatedAt) > e.episodeGap {
		log.Info().
			Str("channel", channelID).
			Str("episode", ep.ID).
			Dur("gap", time.Since(ep.UpdatedAt)).
			Msg("engine: closing stale episode")

		if err := e.closeEpisode(ctx, ep); err != nil {
			log.Error().Err(err).Msg("engine: failed to close episode, continuing with new")
		}

		ep = e.store.NewEpisode(channelID)
		if err := ep.Save(); err != nil {
			return nil, err
		}
		if err := e.store.SetActiveEpisodeMapping(channelID, ep.ID); err != nil {
			return nil, err
		}
	}

	if err := ep.LoadMessages(); err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	return ep, nil
}

func (e *Engine) resetEpisode(ctx context.Context, channelID string) error {
	ep, err := e.store.LoadOrCreateActiveEpisode(channelID)
	if err != nil {
		return err
	}

	if ep.HasMessages() {
		if err := e.closeEpisode(ctx, ep); err != nil {
			log.Error().Err(err).Msg("engine: failed to close episode on reset")
		}
	}

	newEp := e.store.NewEpisode(channelID)
	if err := newEp.Save(); err != nil {
		return err
	}
	return e.store.SetActiveEpisodeMapping(channelID, newEp.ID)
}

func buildScheduledPrompt(taskID, command, skill string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Scheduled Task: %s]\n", taskID)
	b.WriteString("You woke up from a scheduled trigger. Execute the following instruction " +
		"using your tools, memory, and skills. Do not write standalone scripts.\n\n")
	fmt.Fprintf(&b, "Instruction: %s", command)
	if skill != "" {
		fmt.Fprintf(&b, "\nSkill: Read skill %q with skill_read and follow its instructions.", skill)
	}
	return b.String()
}

type episodeSummaryResult struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

func (e *Engine) closeEpisode(ctx context.Context, ep *store2.Episode) error {
	messages, err := ep.TextMessages()
	if err != nil || len(messages) == 0 {
		ep.Close()
		return ep.Save()
	}

	var transcript strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&transcript, "[%s]: %s\n", m.Role, m.Content)
	}

	promptText := "Summarize this conversation episode. Return ONLY valid JSON:\n" +
		`{"title": "short title", "summary": "2-3 sentence summary", "tags": ["tag1", "tag2"]}` +
		"\n\nPreserve: key decisions, outcomes, important facts, action items.\n\n---\n\n" +
		transcript.String()

	msgs := []types.Message{
		types.NewUserMessage(types.TextBlock(promptText)),
	}
	resp, err := e.llmClient.Call(ctx, msgs, llm2.CallOptions{
		System:  "You summarize conversations into structured JSON. Be concise and accurate.",
		NoTools: true,
	})
	if err != nil {
		log.Error().Err(err).Str("episode", ep.ID).Msg("engine: episode summarization failed")
		if len(messages) > 0 {
			ep.Title = store2.TruncTitle(messages[0].Content)
		}
		ep.Summary = "(summarization failed)"
	} else {
		text := types.ExtractText(resp.Content)
		var result episodeSummaryResult
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			start := strings.Index(text, "{")
			end := strings.LastIndex(text, "}")
			if start >= 0 && end > start {
				_ = json.Unmarshal([]byte(text[start:end+1]), &result)
			}
		}
		if result.Title != "" {
			ep.Title = result.Title
		} else if ep.Title == "" && len(messages) > 0 {
			ep.Title = store2.TruncTitle(messages[0].Content)
		}
		ep.Summary = result.Summary
		ep.Tags = result.Tags
	}

	ep.Close()
	return ep.Save()
}
