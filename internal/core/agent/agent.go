package agent

import (
	"gogogot/internal/core/agent/event"
	hook2 "gogogot/internal/core/agent/hook"
	"gogogot/internal/core/agent/prompt"
	"gogogot/internal/llm"
	"gogogot/internal/tools"
	"gogogot/internal/tools/store"
	"gogogot/internal/tools/system"
	"gogogot/internal/tools/types"
)

type Config struct {
	PromptLoader   func() prompt.PromptContext
	Model          string
	MaxTokens      int
	Tools          []string
	Compaction     store.CompactionConfig
	EvalIterations int
}

type Agent struct {
	client       llm.LLM
	config       Config
	registry     *tools.Registry
	localTools   map[string]types.Tool
	loopDetector *hook2.LoopDetector
	beforeHooks  []hook2.BeforeIterationFunc
	afterHooks   []hook2.AfterIterationFunc
	bus          *event.Bus
}

func New(client llm.LLM, config Config, registry *tools.Registry) *Agent {
	tp := system.NewTaskPlan()
	tpTool := system.TaskPlanTool(tp)

	a := &Agent{
		client:   client,
		config:   config,
		registry: registry,
		localTools: map[string]types.Tool{
			tpTool.Name: tpTool,
		},
	}

	a.loopDetector = hook2.NewLoopDetector(0)
	a.AddBeforeHook(hook2.CompactionBeforeIteration(config.Compaction, a.summarize))
	a.AddBeforeHook(hook2.LoggingBeforeIteration())
	a.AddAfterHook(hook2.LoggingAfterIteration())
	a.AddAfterHook(hook2.UsageAfterIteration(
		client.InputPricePerM(), client.OutputPricePerM(),
	))

	return a
}

func (a *Agent) AddBeforeHook(fn hook2.BeforeIterationFunc) {
	a.beforeHooks = append(a.beforeHooks, fn)
}

func (a *Agent) AddAfterHook(fn hook2.AfterIterationFunc) {
	a.afterHooks = append(a.afterHooks, fn)
}

func (a *Agent) ModelLabel() string {
	return a.client.ModelLabel()
}
