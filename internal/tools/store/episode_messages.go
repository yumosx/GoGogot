package store

import (
	"bufio"
	"encoding/json"
	"gogogot/internal/llm/types"
	"os"
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

// --- JSONL I/O ---

func turnToJSON(msg Turn) jsonMessage {
	jm := jsonMessage{
		Role:      msg.Role,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
	}
	if v, ok := msg.Metadata["compacted"].(bool); ok && v {
		jm.Compacted = true
	}
	return jm
}

func (e *Episode) appendToJSONL(msg Turn) {
	path := e.MessagesPath()
	if path == "" {
		return
	}
	line, err := json.Marshal(turnToJSON(msg))
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
		line, err := json.Marshal(turnToJSON(msg))
		if err != nil {
			continue
		}
		f.Write(line)
		f.Write([]byte{'\n'})
	}
	return nil
}

