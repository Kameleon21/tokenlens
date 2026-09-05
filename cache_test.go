package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotCache(t *testing.T) {
	m := fixtureModel()
	o := m.o
	o.Demo = false
	o.CacheDir = t.TempDir()
	if e := writeSnapshotCache(o, o.Range, m.s); e != nil {
		t.Fatal(e)
	}
	s, e := readSnapshotCache(o, o.Range)
	if e != nil || len(s.Sections["daily"]) != 30 {
		t.Fatal(e)
	}
	path, _ := snapshotCachePath(o, o.Range)
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("cache not private")
	}
	other := o
	other.TZ = "Europe/Dublin"
	if _, e = readSnapshotCache(other, o.Range); e == nil {
		t.Fatal("timezone leaked between caches")
	}
	other = o
	other.NoCache = true
	if _, e = readSnapshotCache(other, o.Range); e == nil {
		t.Fatal("no-cache ignored")
	}
	m.s.Loaded = time.Now().Add(-8 * 24 * time.Hour)
	_ = writeSnapshotCache(o, o.Range, m.s)
	if _, e = readSnapshotCache(o, o.Range); e == nil {
		t.Fatal("expired cache accepted")
	}
	_ = os.WriteFile(path, []byte("corrupt"), 0600)
	if _, e = readSnapshotCache(o, o.Range); e == nil {
		t.Fatal("corrupt cache accepted")
	}
}
func TestCachedSnapshotDoesNotOverwriteFresh(t *testing.T) {
	m := fixtureModel()
	m.request = 3
	m.loading = true
	v, _ := m.Update(cachedMsg{m.s, m.o.Range, 3})
	n := v.(model)
	if !n.cached {
		t.Fatal("cache not shown while loading")
	}
	n.loading = false
	n.cached = false
	v, _ = n.Update(cachedMsg{m.s, m.o.Range, 3})
	if v.(model).cached {
		t.Fatal("cache overwrote completed refresh")
	}
}
func TestDemoDoesNotPersist(t *testing.T) {
	m := fixtureModel()
	m.o.CacheDir = t.TempDir()
	if e := writeSnapshotCache(m.o, m.o.Range, m.s); e != nil {
		t.Fatal(e)
	}
	files, _ := filepath.Glob(filepath.Join(m.o.CacheDir, "*"))
	if len(files) != 0 {
		t.Fatal("demo persisted")
	}
}
func TestWarmLoadIntegration(t *testing.T) {
	if os.Getenv("TOKENLENS_TEST_WARM_CACHE") == "" {
		t.Skip("opt-in real report")
	}
	o, e := options([]string{"--since", "20260901", "--until", "20260930", "--timezone", "UTC"}, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s, e := load(ctx, o.Bin, o.Range, o.TZ)
	if e != nil {
		t.Fatal(e)
	}
	if e = writeSnapshotCache(o, o.Range, s); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	_, e = readSnapshotCache(o, o.Range)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("saved real snapshot; cache read %s", time.Since(start))
}
