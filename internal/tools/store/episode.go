package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gogogot/internal/infra/utils"
	"gogogot/internal/llm/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Message is a text-only representation used for summarization and history display.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func TruncTitle(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return utils.Truncate(s, 60, "...")
}

type Episode struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Tags      []string  `json:"tags"`
	Status    string    `json:"status"` // "active" | "closed"
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`

	// In-memory state (not serialized to episode JSON).
	episodesDir string `json:"-"`
	messages    []Turn `json:"-"`
	totalUsage  Usage  `json:"-"`
}

type EpisodeInfo struct {
	ID        string
	Title     string
	Summary   string
	Tags      []string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
}

func (s *Store) NewEpisode(channelID string) *Episode {
	now := time.Now()
	return &Episode{
		ID:          uuid.NewString(),
		ChannelID:   channelID,
		Status:      "active",
		StartedAt:   now,
		UpdatedAt:   now,
		episodesDir: s.episodesDir(),
	}
}

func (e *Episode) path() string {
	date := e.StartedAt.Format("2006-01-02")
	return filepath.Join(e.episodesDir, date, e.ID+".json")
}

func (e *Episode) Save() error {
	dir := filepath.Dir(e.path())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	e.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.path(), data, 0o644)
}

// MessagesPath returns the path to the JSONL messages file for this episode.
func (e *Episode) MessagesPath() string {
	date := e.StartedAt.Format("2006-01-02")
	return filepath.Join(e.episodesDir, date, e.ID+".messages.jsonl")
}

func (e *Episode) String() string { return e.ID }

func (e *Episode) Close() {
	e.Status = "closed"
	e.EndedAt = time.Now()
}

// TextMessages reads the messages JSONL and returns text-only messages
// suitable for summarization and transcript generation.
func (e *Episode) TextMessages() ([]Message, error) {
	f, err := os.Open(e.MessagesPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var jm struct {
			Role    string               `json:"role"`
			Content []types.ContentBlock `json:"content"`
		}
		if json.Unmarshal(line, &jm) != nil {
			continue
		}
		text := types.ExtractText(jm.Content)
		if text == "" {
			continue
		}
		msgs = append(msgs, Message{Role: jm.Role, Content: text})
	}
	return msgs, scanner.Err()
}

// HasMessages returns true if the episode has a non-empty messages file.
func (e *Episode) HasMessages() bool {
	info, err := os.Stat(e.MessagesPath())
	return err == nil && info.Size() > 0
}

func (s *Store) LoadEpisode(id string) (*Episode, error) {
	fname := id + ".json"
	entries, err := os.ReadDir(s.episodesDir())
	if err != nil {
		return nil, fmt.Errorf("episode %q not found", id)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.episodesDir(), entry.Name(), fname))
		if err != nil {
			continue
		}
		var ep Episode
		if err := json.Unmarshal(data, &ep); err != nil {
			return nil, err
		}
		ep.episodesDir = s.episodesDir()
		return &ep, nil
	}
	return nil, fmt.Errorf("episode %q not found", id)
}

func (s *Store) ListEpisodes() ([]EpisodeInfo, error) {
	dateDirs, err := os.ReadDir(s.episodesDir())
	if err != nil {
		return nil, err
	}
	var out []EpisodeInfo
	for _, dd := range dateDirs {
		if !dd.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(s.episodesDir(), dd.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") || strings.HasSuffix(f.Name(), ".messages.jsonl") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(s.episodesDir(), dd.Name(), f.Name()))
			if err != nil {
				continue
			}
			var ep Episode
			if json.Unmarshal(data, &ep) == nil {
				out = append(out, EpisodeInfo{
					ID:        ep.ID,
					Title:     ep.Title,
					Summary:   ep.Summary,
					Tags:      ep.Tags,
					Status:    ep.Status,
					StartedAt: ep.StartedAt,
					EndedAt:   ep.EndedAt,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}

func (s *Store) SearchEpisodes(query string) ([]EpisodeInfo, error) {
	all, err := s.ListEpisodes()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	words := strings.Fields(q)
	if len(words) == 0 {
		return nil, nil
	}

	var matches []EpisodeInfo
	for _, ep := range all {
		if ep.Status != "closed" || ep.Summary == "" {
			continue
		}
		corpus := strings.ToLower(ep.Title + " " + ep.Summary + " " + strings.Join(ep.Tags, " "))
		matched := false
		for _, w := range words {
			if strings.Contains(corpus, w) {
				matched = true
				break
			}
		}
		if matched {
			matches = append(matches, ep)
		}
	}

	const maxResults = 5
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}
	return matches, nil
}
