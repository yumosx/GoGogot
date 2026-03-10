package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gogogot/internal/llm/types"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Turn is a single message in the LLM conversation context.
// Rich format: includes tool_use, tool_result, images — everything the LLM sees.
type Turn struct {
	Role      string // "user" | "assistant"
	Content   []types.ContentBlock
	Timestamp time.Time
	Usage     *Usage
	Metadata  map[string]any
}

type jsonMessage struct {
	Role      string               `json:"role"`
	Content   []types.ContentBlock `json:"content"`
	Timestamp time.Time            `json:"ts"`
	Compacted bool                 `json:"compacted,omitempty"`
}

// Usage tracks token consumption and cost for a run.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	TotalTokens      int
	LLMCalls         int
	ToolCalls        int
	Cost             float64 // estimated USD
	Duration         time.Duration
}

func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.CacheWriteTokens += other.CacheWriteTokens
	u.TotalTokens += other.TotalTokens
	u.LLMCalls += other.LLMCalls
	u.ToolCalls += other.ToolCalls
	u.Cost += other.Cost
	u.Duration += other.Duration
}

const charsPerToken = 4

func EstimateTokens(messages []Turn) int {
	var chars int
	for _, m := range messages {
		chars += estimateBlocksChars(m.Content)
	}
	return chars / charsPerToken
}

func estimateBlocksChars(blocks []types.ContentBlock) int {
	var n int
	for _, b := range blocks {
		switch b.Type {
		case "text":
			n += len(b.Text)
		case "tool_use":
			n += len(b.ToolName) + len(b.ToolInput)
		case "tool_result":
			n += len(b.ToolOutput)
		case "image":
			n += 1000
		}
	}
	return n
}

// CompactionConfig controls when and how episode messages are compacted.
type CompactionConfig struct {
	Threshold      float64 // 0.0–1.0, fraction of context window that triggers compaction
	SafetyMargin   float64 // 1.2 = 20% buffer for token estimate inaccuracy
	PreserveRecent int     // number of recent messages to keep uncompressed
	SummaryPrompt  string  // instruction for the summarization LLM call
}

func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		Threshold:      0.8,
		SafetyMargin:   1.2,
		PreserveRecent: 5,
		SummaryPrompt:  "Summarize the conversation so far. Preserve decisions, TODOs, constraints, errors, file paths mentioned, the current plan, and task_plan checklist state (task IDs, titles, statuses).",
	}
}

func (cc *CompactionConfig) ShouldCompact(estimatedTokens, contextWindow int) bool {
	if contextWindow <= 0 || cc.Threshold <= 0 {
		return false
	}
	adjusted := float64(estimatedTokens) * cc.SafetyMargin
	limit := cc.Threshold * float64(contextWindow)
	return adjusted > limit
}

// --- Episode message methods ---

// LoadMessages reads the JSONL file into in-memory messages.
func (e *Episode) LoadMessages() error {
	e.messages = make([]Turn, 0)
	f, err := os.Open(e.MessagesPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var jm jsonMessage
		if err := json.Unmarshal(line, &jm); err != nil {
			log.Warn().Err(err).Msg("episode: skipping corrupt JSONL line")
			continue
		}
		msg := Turn{
			Role:      jm.Role,
			Content:   jm.Content,
			Timestamp: jm.Timestamp,
		}
		if jm.Compacted {
			msg.Metadata = map[string]any{"compacted": true}
		}
		e.messages = append(e.messages, msg)
	}
	return scanner.Err()
}

// AppendMessage adds a message to the episode and persists it to the JSONL file.
func (e *Episode) AppendMessage(msg Turn) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Usage != nil {
		e.totalUsage.Add(*msg.Usage)
	}
	e.messages = append(e.messages, msg)
	e.appendToJSONL(msg)
}

// Messages returns the in-memory episode messages.
func (e *Episode) Messages() []Turn {
	return e.messages
}

// TotalUsage returns a pointer to the accumulated usage for this run.
func (e *Episode) TotalUsage() *Usage {
	return &e.totalUsage
}

// ReplaceMessages replaces in-memory messages and rewrites the JSONL file.
func (e *Episode) ReplaceMessages(msgs []Turn) error {
	e.messages = msgs
	return e.rewriteJSONL()
}

// CompactAll replaces all messages with a single compaction notice.
func (e *Episode) CompactAll(reason string) {
	e.messages = []Turn{
		{
			Role:      string(types.RoleAssistant),
			Content:   []types.ContentBlock{types.TextBlock("Context compacted. Reason: " + reason)},
			Timestamp: time.Now(),
		},
	}
	if err := e.rewriteJSONL(); err != nil {
		log.Error().Err(err).Msg("episode: failed to rewrite JSONL after CompactAll")
	}
}

// --- JSONL I/O ---

func (e *Episode) appendToJSONL(msg Turn) {
	path := e.MessagesPath()
	if path == "" {
		return
	}
	jm := jsonMessage{
		Role:      msg.Role,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
	}
	if msg.Metadata != nil {
		if v, ok := msg.Metadata["compacted"]; ok && v == true {
			jm.Compacted = true
		}
	}
	line, err := json.Marshal(jm)
	if err != nil {
		log.Error().Err(err).Msg("episode: failed to marshal message for JSONL")
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Error().Err(err).Msg("episode: failed to open JSONL for append")
		return
	}
	defer f.Close()
	f.Write(line)
	f.Write([]byte{'\n'})
}

func (e *Episode) rewriteJSONL() error {
	path := e.MessagesPath()
	if path == "" {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, msg := range e.messages {
		jm := jsonMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		}
		if msg.Metadata != nil {
			if v, ok := msg.Metadata["compacted"]; ok && v == true {
				jm.Compacted = true
			}
		}
		line, err := json.Marshal(jm)
		if err != nil {
			continue
		}
		f.Write(line)
		f.Write([]byte{'\n'})
	}
	return nil
}

// --- Transcript helpers (for compaction) ---

// RenderTranscript serializes messages into a human-readable transcript.
func RenderTranscript(msgs []Turn) string {
	var sb strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&sb, "[%s]: ", m.Role)
		sb.WriteString(contentToString(m.Content))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func contentToString(blocks []types.ContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		switch b.Type {
		case "text":
			sb.WriteString(b.Text)
		case "tool_use":
			fmt.Fprintf(&sb, "[tool_use: %s]", b.ToolName)
		case "tool_result":
			sb.WriteString(b.ToolOutput)
		case "image":
			sb.WriteString("[image]")
		}
	}
	return sb.String()
}
