package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aioproxy/aioproxy/internal/core"
)

const StateVersion = 1

type Store struct {
	dataDir   string
	retention int
	mu        sync.Mutex
}

func New(dataDir string, retention int) *Store {
	if retention <= 0 {
		retention = 7
	}
	return &Store{dataDir: dataDir, retention: retention}
}

type poolState struct {
	Version    int              `json:"version"`
	SavedAt    time.Time        `json:"saved_at"`
	Candidates []core.Candidate `json:"candidates"`
}

func (s *Store) LoadPool() ([]core.Candidate, error) {
	path := filepath.Join(s.dataDir, "pool.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st poolState
	if err := json.Unmarshal(b, &st); err != nil || st.Version != StateVersion {
		backup := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
		_ = os.Rename(path, backup)
		if err != nil {
			return nil, fmt.Errorf("pool state incompatible; backed up to %s: %w", backup, err)
		}
		return nil, fmt.Errorf("pool state version %d incompatible; backed up to %s", st.Version, backup)
	}
	for i := range st.Candidates {
		st.Candidates[i].Normalize()
		if st.Candidates[i].Protocol == core.ProtocolSingBox {
			st.Candidates[i].Status = core.StatusUnavailable
			st.Candidates[i].LastError = "sing-box persisted candidate requires plugin refresh"
		}
	}
	return st.Candidates, nil
}

func (s *Store) SavePool(candidates []core.Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}
	st := poolState{Version: StateVersion, SavedAt: time.Now(), Candidates: candidates}
	return writeJSONAtomic(filepath.Join(s.dataDir, "pool.json"), st)
}

func (s *Store) SaveSnapshot(source string, report core.ImportReport, candidates []core.Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	safe := safeName(source)
	dir := filepath.Join(s.dataDir, "snapshots", safe)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload := struct {
		Version    int               `json:"version"`
		SavedAt    time.Time         `json:"saved_at"`
		Report     core.ImportReport `json:"report"`
		Candidates []core.Candidate  `json:"candidates"`
	}{StateVersion, time.Now(), report, candidates}
	name := time.Now().Format("20060102-150405.000000000") + ".json"
	if err := writeJSONAtomic(filepath.Join(dir, name), payload); err != nil {
		return err
	}
	return s.prune(dir)
}

func (s *Store) SnapshotFiles() ([]string, error) {
	root := filepath.Join(s.dataDir, "snapshots")
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, nil
}

func (s *Store) prune(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	for len(files) > s.retention {
		_ = os.Remove(files[0])
		files = files[1:]
	}
	return nil
}

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func safeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(s)
}
