package core

import (
	"context"
	"fmt"
	"gogogot/internal/channel"
	"gogogot/internal/core/agent"
	event2 "gogogot/internal/core/agent/event"
	"gogogot/internal/core/agent/hook"
	"gogogot/internal/core/episode"
	"gogogot/internal/core/prompt"
	transport2 "gogogot/internal/core/transport"
	"gogogot/internal/infra/config"
	"gogogot/internal/infra/scheduler"
	llm2 "gogogot/internal/llm"
	"gogogot/internal/llm/types"
	"gogogot/internal/tools"
	store2 "gogogot/internal/tools/store"
	"gogogot/internal/tools/system"
	"sync"

	"github.com/rs/zerolog/log"
)

type Engine struct {
	ch        channel.Channel
	agent     *agent.Agent
	store     *store2.Store
	episodes  *episode.Manager
	scheduler *scheduler.Scheduler
	registry  *tools.Registry

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

	extra := append(transport2.ChannelTools(),
		system.ScheduleTools(sched)...,
	)
	extra = append(extra, st.IdentityTools(sched.SetLocation)...)

	client := llm2.NewClient(*provider, nil)
	epMgr := episode.NewManager(st, client)

	reg := tools.NewRegistry(st, cfg.BraveAPIKey, epMgr.SearchRelevant, extra...)

	client.SetTools(reg.Definitions())

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
		Compaction: hook.DefaultCompactionConfig(),
	}

	eng := &Engine{
		ch:        ch,
		store:     st,
		episodes:  epMgr,
		scheduler: sched,
		registry:  reg,
		cancels:   make(map[string]context.CancelFunc),
	}
	eng.agent = agent.New(client, agentCfg, reg)

	ownerSessionID, ownerReply := ch.OwnerSession()
	sched.SetExecutor(func(ctx context.Context, taskID, command, skill string) (string, error) {
		return eng.RunScheduledTask(ctx, ownerSessionID, ownerReply, taskID, command, skill)
	})

	return eng, nil
}

func (e *Engine) Run(ctx context.Context) error {
	if err := e.scheduler.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer e.scheduler.Stop()

	return e.ch.Run(ctx, e.handleMessage)
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

func (e *Engine) Channel() channel.Channel {
	return e.ch
}

func (e *Engine) handleMessage(ctx context.Context, msg channel.Message) {
	if msg.Command != nil {
		e.handleCommand(ctx, msg)
		return
	}

	e.mu.Lock()
	_, busy := e.cancels[msg.SessionID]
	e.mu.Unlock()

	if busy {
		_ = msg.Reply.SendText(ctx, "Still working on the previous task, please wait...")
		return
	}

	if msg.Text == "" && len(msg.Attachments) == 0 {
		return
	}

	log.Info().
		Str("session", msg.SessionID).
		Int("text_len", len(msg.Text)).
		Int("attachments", len(msg.Attachments)).
		Msg("engine: incoming message")

	go e.runAgent(ctx, msg)
}

func (e *Engine) handleCommand(ctx context.Context, msg channel.Message) {
	cmd := msg.Command
	switch cmd.Name {
	case channel.CmdNewEpisode:
		cmd.Result.Error = e.episodes.Reset(ctx, msg.SessionID)
	case channel.CmdStop:
		e.stopAgent(msg.SessionID, cmd)
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

func (e *Engine) runAgent(ctx context.Context, msg channel.Message) {
	reply := msg.Reply
	sessionID := msg.SessionID

	agentCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	e.mu.Lock()
	e.cancels[sessionID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, sessionID)
		e.mu.Unlock()
	}()

	ep, err := e.episodes.Resolve(agentCtx, sessionID, msg.Text)
	if err != nil {
		log.Error().Err(err).Msg("engine: failed to resolve episode")
		_ = reply.SendText(ctx, "Error: "+err.Error())
		return
	}

	_ = reply.SendTyping(ctx)
	statusID, _ := reply.SendStatus(ctx, channel.AgentStatus{Phase: channel.PhaseThinking})

	agentCtx = channel.WithReplier(agentCtx, reply)

	blocks, cleanup := transport2.ProcessAttachments(ep.ID, msg.Text, msg.Attachments)
	defer cleanup()

	bus, recv := event2.NewBus(64)
	go func() {
		defer bus.Close()
		if err := e.agent.Run(agentCtx, ep, blocks, bus); err != nil {
			log.Error().Err(err).Str("session", sessionID).Msg("engine: agent run failed")
		}
	}()

	finalText := transport2.ConsumeEvents(agentCtx, reply, recv, statusID)
	if finalText != "" {
		_ = reply.SendText(ctx, finalText)
	}
}

func (e *Engine) stopAgent(sessionID string, cmd *channel.Command) {
	e.mu.Lock()
	cancel, running := e.cancels[sessionID]
	e.mu.Unlock()

	if !running {
		cmd.Result.Data = map[string]string{"text": "Nothing to cancel."}
		return
	}

	cancel()
	cmd.Result.Data = map[string]string{"text": "⏹ Stopping..."}
}

// RunScheduledTask executes a scheduled task in the active episode for the
// given session. It runs synchronously and returns the agent's text output.
// If the agent is already busy on this session, it returns an error so the
// scheduler can apply backoff and retry later.
func (e *Engine) RunScheduledTask(ctx context.Context, sessionID string, reply channel.Replier, taskID, command, skill string) (string, error) {
	e.mu.Lock()
	_, busy := e.cancels[sessionID]
	e.mu.Unlock()
	if busy {
		return "", fmt.Errorf("agent busy on session %s, will retry", sessionID)
	}

	agentCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	agentCtx = channel.WithReplier(agentCtx, reply)

	e.mu.Lock()
	e.cancels[sessionID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, sessionID)
		e.mu.Unlock()
	}()

	ep, err := e.episodes.Resolve(agentCtx, sessionID, command)
	if err != nil {
		return "", fmt.Errorf("resolve episode: %w", err)
	}

	promptText := prompt.ScheduledTaskPrompt(taskID, command, skill)
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
		_ = reply.SendText(ctx, finalText)
	}

	return finalText, nil
}

