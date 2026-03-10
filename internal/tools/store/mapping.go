package store

import (
	"encoding/json"
	"fmt"
	"os"
)

func (s *Store) loadEpisodeMapping() (map[string]string, error) {
	data, err := os.ReadFile(s.episodeMappingPath())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) saveEpisodeMapping(m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.episodeMappingPath(), data, 0o644)
}

func (s *Store) LoadOrCreateActiveEpisode(channelID string) (*Episode, error) {
	m, err := s.loadEpisodeMapping()
	if err != nil {
		return nil, err
	}
	if epID, ok := m[channelID]; ok {
		ep, err := s.LoadEpisode(epID)
		if err == nil && ep.Status == "active" {
			return ep, nil
		}
	}
	ep := s.NewEpisode(channelID)
	if err := ep.Save(); err != nil {
		return nil, err
	}
	m[channelID] = ep.ID
	if err := s.saveEpisodeMapping(m); err != nil {
		return nil, err
	}
	return ep, nil
}

func (s *Store) GetActiveEpisodeMapping(channelID string) (string, error) {
	m, err := s.loadEpisodeMapping()
	if err != nil {
		return "", err
	}
	epID, ok := m[channelID]
	if !ok {
		return "", fmt.Errorf("no episode mapping for %q", channelID)
	}
	return epID, nil
}

func (s *Store) SetActiveEpisodeMapping(channelID, episodeID string) error {
	m, err := s.loadEpisodeMapping()
	if err != nil {
		return err
	}
	m[channelID] = episodeID
	return s.saveEpisodeMapping(m)
}
