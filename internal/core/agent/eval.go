package agent

import (
	"context"
	"fmt"
	"gogogot/internal/core/agent/event"
	"gogogot/internal/core/agent/hook"
	"gogogot/internal/llm/types"

	"github.com/rs/zerolog/log"
)

type EvalResult struct {
	Passed   bool
	Score    float64
	Feedback string
	Details  map[string]any
}

type Evaluator interface {
	Evaluate(ctx context.Context) EvalResult
}

func (a *Agent) RunWithEval(ctx context.Context, conv hook.Conversation, task string, eval Evaluator, bus *event.Bus) error {
	maxIters := a.config.EvalIterations
	if maxIters <= 0 {
		maxIters = 1
	}

	for iter := 0; iter < maxIters; iter++ {
		err := a.Run(ctx, conv, []types.ContentBlock{types.TextBlock(task)}, bus)
		if err != nil {
			return err
		}

		if eval == nil {
			break
		}

		log.Info().Int("iteration", iter).Msg("eval: running evaluator")
		result := eval.Evaluate(ctx)
		log.Info().
			Int("iteration", iter).
			Bool("passed", result.Passed).
			Str("feedback", result.Feedback).
			Msg("eval: result")

		if result.Passed {
			return nil
		}

		conv.CompactAll("attempt failed: " + result.Feedback)
		task = fmt.Sprintf("Previous attempt failed.\nFeedback: %s\nOriginal task: %s", result.Feedback, task)
	}

	if eval != nil {
		return fmt.Errorf("eval loop: max iterations (%d) reached", maxIters)
	}
	return nil
}
