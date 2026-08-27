package store

import (
	"path/filepath"
	"sync"
	"testing"

	"task284-boreprobe/internal/model"
)

func TestConcurrentAdjudicateImpactOptimisticLock(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	p, _ := db.CreateProject("并发裁决", "oboe", 14.5)
	patch, err := db.CreatePatch(&model.Patch{ProjectID: p.ID, AxialStart: 10, AxialEnd: 20, ThicknessMM: 1.2, Material: "枫木"})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := db.CreateImpact(&model.RepairImpact{ProjectID: p.ID, PatchID: patch.ID})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	stale := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		kind := model.ImpactNone
		if i%2 == 0 {
			kind = model.ImpactNarrowed
		}
		go func(k string) {
			defer wg.Done()
			_, err := db.AdjudicateImpact(imp.ID, k, imp.Version)
			mu.Lock()
			defer mu.Unlock()
			if err == model.ErrVersionStale {
				stale++
				return
			}
			if err != nil {
				t.Errorf("adjudicate: %v", err)
				return
			}
			success++
		}(kind)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("期望 1 次成功，实际 %d (stale=%d)", success, stale)
	}
	if stale != workers-1 {
		t.Fatalf("期望 %d 次 stale，实际 %d", workers-1, stale)
	}
	got, err := db.GetImpact(imp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != imp.Version+1 {
		t.Fatalf("version 应为 %d，实际 %d", imp.Version+1, got.Version)
	}
}
