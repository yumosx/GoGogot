package store

import (
	"os"
	"path/filepath"
)

type Store struct {
	dataDir string
}

func New(dataDir string) (*Store, error) {
	s := &Store{dataDir: dataDir}
	if err := s.ensureDirs(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) DataDir() string { return s.dataDir }

func (s *Store) episodesDir() string        { return filepath.Join(s.dataDir, "episodes") }
func (s *Store) memoryDir() string          { return filepath.Join(s.dataDir, "memory") }
func (s *Store) SkillsDir() string          { return filepath.Join(s.dataDir, "skills") }
func (s *Store) episodeMappingPath() string { return filepath.Join(s.dataDir, "external_episodes.json") }

func (s *Store) ensureDirs() error {
	for _, dir := range []string{s.episodesDir(), s.memoryDir(), s.SkillsDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
