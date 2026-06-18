package storage

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aioproxy/aioproxy/internal/core"
)

func TestConcurrentSavePool(t *testing.T) {
	store := New(t.TempDir(), 2)
	candidates := []core.Candidate{{Protocol: core.ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "test"}}
	var wg sync.WaitGroup
	errCh := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- store.SavePool(candidates)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("SavePool failed: %v", err)
		}
	}
	loaded, err := store.LoadPool()
	if err != nil {
		t.Fatalf("LoadPool failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded=%d", len(loaded))
	}
	matches, err := filepath.Glob(filepath.Join(store.dataDir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("tmp files left: %v", matches)
	}
}

func TestConcurrentSaveSnapshotRetention(t *testing.T) {
	store := New(t.TempDir(), 3)
	report := core.ImportReport{Plugin: "test", Source: "src"}
	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- store.SaveSnapshot("test-src", report, nil)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("SaveSnapshot failed: %v", err)
		}
	}
	files, err := store.SnapshotFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("snapshot files=%d want=3: %v", len(files), files)
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".tmp") {
			t.Fatalf("tmp snapshot retained: %s", f)
		}
	}
}
