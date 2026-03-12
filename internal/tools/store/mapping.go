package store

import (
	"fmt"
	"os"
	"strings"
)

func (s *Store) loadActiveEpisodeID() (string, error) {
	data, err := os.ReadFile(s.activeEpisodePath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Store) saveActiveEpisodeID(id string) error {
	return os.WriteFile(s.activeEpisodePath(), []byte(id+"\n"), 0o644)
}

func (s *Store) LoadOrCreateActiveEpisode() (*Episode, error) {
	epID, err := s.loadActiveEpisodeID()
	if err != nil {
		return nil, err
	}
	if epID != "" {
		ep, err := s.LoadEpisode(epID)
		if err == nil && ep.Status == "active" {
			return ep, nil
		}
	}
	ep := s.NewEpisode()
	if err := ep.Save(); err != nil {
		return nil, err
	}
	if err := s.saveActiveEpisodeID(ep.ID); err != nil {
		return nil, err
	}
	return ep, nil
}

func (s *Store) GetActiveEpisodeID() (string, error) {
	epID, err := s.loadActiveEpisodeID()
	if err != nil {
		return "", err
	}
	if epID == "" {
		return "", fmt.Errorf("no active episode")
	}
	return epID, nil
}

func (s *Store) SetActiveEpisodeID(episodeID string) error {
	return s.saveActiveEpisodeID(episodeID)
}
