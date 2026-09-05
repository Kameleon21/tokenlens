package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Kameleon21/tokenlens/internal/datefilter"
	"os"
	"path/filepath"
	"time"
)

type cachedMsg struct {
	s  Snapshot
	r  datefilter.Range
	id int
}
type diskSnapshot struct {
	Version  int
	Snapshot Snapshot
}

func snapshotCachePath(o Options, r datefilter.Range) (string, error) {
	dir := o.CacheDir
	if dir == "" {
		root, e := os.UserCacheDir()
		if e != nil {
			return "", e
		}
		dir = filepath.Join(root, "tokenlens")
	}
	key := sha256.Sum256([]byte("v1\x00" + o.Bin + fmt.Sprint(o.Offline) + "\x00" + o.TZ + "\x00" + r.Since + "\x00" + r.Until))
	return filepath.Join(dir, hex.EncodeToString(key[:])+".json"), nil
}
func readSnapshotCache(o Options, r datefilter.Range) (Snapshot, error) {
	if o.NoCache || o.Demo {
		return Snapshot{}, os.ErrNotExist
	}
	path, e := snapshotCachePath(o, r)
	if e != nil {
		return Snapshot{}, e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return Snapshot{}, e
	}
	var entry diskSnapshot
	if e = json.Unmarshal(b, &entry); e != nil {
		return Snapshot{}, e
	}
	if entry.Version != 1 || entry.Snapshot.Loaded.IsZero() || entry.Snapshot.Loaded.After(time.Now().Add(time.Minute)) {
		return Snapshot{}, fmt.Errorf("invalid cached snapshot")
	}
	if time.Since(entry.Snapshot.Loaded) > 7*24*time.Hour {
		return Snapshot{}, fmt.Errorf("cached snapshot expired")
	}
	return entry.Snapshot, nil
}
func writeSnapshotCache(o Options, r datefilter.Range, s Snapshot) error {
	if o.NoCache || o.Demo {
		return nil
	}
	path, e := snapshotCachePath(o, r)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, e := json.Marshal(diskSnapshot{Version: 1, Snapshot: s})
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

// reusedMsg completes a request without invoking the backend.
type reusedMsg struct {
	s  Snapshot
	r  datefilter.Range
	id int
}

func snapshotFresh(s Snapshot, ttl time.Duration, now time.Time) bool {
	age := now.Sub(s.Loaded)
	return ttl > 0 && !s.Loaded.IsZero() && age >= 0 && age < ttl
}

func (m *model) remember(r datefilter.Range, s Snapshot) {
	if m.o.NoCache || m.o.Demo {
		return
	}
	if m.reports == nil {
		m.reports = make(map[datefilter.Range]Snapshot)
	}
	if _, exists := m.reports[r]; !exists && len(m.reports) >= 16 {
		var oldest datefilter.Range
		var loaded time.Time
		for key, report := range m.reports {
			if loaded.IsZero() || report.Loaded.Before(loaded) {
				oldest, loaded = key, report.Loaded
			}
		}
		delete(m.reports, oldest)
	}
	m.reports[r] = s
}
