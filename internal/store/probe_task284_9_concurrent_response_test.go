package store

import (
	"path/filepath"
	"sync"
	"testing"

	"task284-boreprobe/internal/model"
)

func TestConcurrentResponseImportIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	p, err := db.CreateProject("并发频响", "oboe", 14.5)
	if err != nil {
		t.Fatal(err)
	}
	resp := &model.FrequencyResponse{
		ProjectID: p.ID, Kind: "before", BaseFreqHz: 440,
		Peaks: []model.Peak{{FreqHz: 880, Amplitude: 0.4}, {FreqHz: 1320, Amplitude: 0.2}},
	}
	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copyResp := *resp
			copyResp.Peaks = append([]model.Peak(nil), resp.Peaks...)
			_, err := db.CreateResponse(&copyResp)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发导入频响失败: %v", err)
		}
	}
	var n int
	if err := db.Raw.QueryRow(`SELECT COUNT(*) FROM responses WHERE project_id=?`, p.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("幂等失败: 期望 1 条频响，实际 %d", n)
	}
}
