package event

import (
	"gogogot/internal/tools/store"
	"time"
)

type Kind string

const (
	LLMStart    Kind = "llm_start"
	LLMResponse Kind = "llm_response"
	LLMStream   Kind = "llm_stream"
	ToolStart   Kind = "tool_start"
	ToolEnd     Kind = "tool_end"
	Compaction  Kind = "compaction"
	LoopWarning Kind = "loop_warning"
	Error       Kind = "error"
	Done        Kind = "done"
)

type Event struct {
	Timestamp time.Time
	Kind      Kind
	Data      any
}

type LLMResponseData struct {
	Usage store.Usage
}

type LLMStreamData struct {
	Text string
}

type ToolStartData struct {
	Name   string
	Detail string
}

type ToolEndData struct {
	Name       string
	Result     string
	DurationMs int64
}

type ErrorData struct {
	Error string
}

type DoneData struct {
	Usage store.Usage
}

type CompactionData struct {
	BeforeTokens int
	AfterTokens  int
}

type LoopWarningData struct {
	Name   string
	Reason string
}
