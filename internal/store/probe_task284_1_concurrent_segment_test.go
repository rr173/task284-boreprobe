package store

import (
	"path/filepath"
	"sync"
	"testing"

	"task284-boreprobe/internal/model"
)

func TestConcurrentSegmentImportIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	p, err := db.CreateProject("并发段导入", "oboe", 14.5)
	if err != nil {
		t.Fatal(err)
	}
	seg := &model.BoreSegment{
		ProjectID: p.ID, Label: "上节", AxialStart: 0, AxialEnd: 50,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{14.5, 14.4, 14.3, 14.2},
	}
	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copySeg := *seg
			copySeg.DiameterMM = append([]float64(nil), seg.DiameterMM...)
			_, err := db.CreateSegment(&copySeg)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发导入失败: %v", err)
		}
	}
	var n int
	if err := db.Raw.QueryRow(`SELECT COUNT(*) FROM segments WHERE project_id=?`, p.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("幂等失败: 期望 1 条段记录，实际 %d", n)
	}
}
