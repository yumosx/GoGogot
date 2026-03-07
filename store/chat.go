package store

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Chat struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func NewChat() *Chat {
	return &Chat{
		ID:        newUUID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (c *Chat) path() string {
	date := c.CreatedAt.Format("2006-01-02")
	return filepath.Join(chatsDir(), date, c.ID+".json")
}

func (c *Chat) Save() error {
	if err := ensureDirs(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path()), 0o755); err != nil {
		return err
	}
	c.UpdatedAt = time.Now()
	if c.Title == "" && len(c.Messages) > 0 {
		c.Title = truncTitle(c.Messages[0].Content)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(), data, 0o644)
}

func LoadChat(id string) (*Chat, error) {
	fname := id + ".json"
	entries, err := os.ReadDir(chatsDir())
	if err != nil {
		return nil, fmt.Errorf("chat %q not found", id)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(chatsDir(), e.Name(), fname))
		if err != nil {
			continue
		}
		var c Chat
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil
	}
	return nil, fmt.Errorf("chat %q not found", id)
}

type ChatInfo struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

func ListChats() ([]ChatInfo, error) {
	if err := ensureDirs(); err != nil {
		return nil, err
	}
	dateDirs, err := os.ReadDir(chatsDir())
	if err != nil {
		return nil, err
	}
	var out []ChatInfo
	for _, dd := range dateDirs {
		if !dd.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(chatsDir(), dd.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(chatsDir(), dd.Name(), f.Name()))
			if err != nil {
				continue
			}
			var c Chat
			if json.Unmarshal(data, &c) == nil {
				out = append(out, ChatInfo{ID: c.ID, Title: c.Title, UpdatedAt: c.UpdatedAt})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func truncTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 60 {
		s = s[:60] + "..."
	}
	return s
}
