package service

import (
	"path/filepath"
	"testing"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db), db
}

func mustProject(t *testing.T, db *store.DB) int64 {
	t.Helper()
	p, err := db.CreateProject("probe-project", "oboe", 14.5)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestPublishSnapshotUsesFreshBaseHash(t *testing.T) {
	svc, db := newTestService(t)
	pid := mustProject(t, db)
	seg := &model.BoreSegment{
		ProjectID: pid, Label: "A", AxialStart: 0, AxialEnd: 10,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{14.5, 14.5, 14.5},
	}
	if _, err := db.CreateSegment(seg); err != nil {
		t.Fatal(err)
	}
	before := &model.FrequencyResponse{
		ProjectID: pid, Kind: "before", BaseFreqHz: 440,
		Peaks: []model.Peak{{FreqHz: 880, Amplitude: 0.5}},
	}
	if _, err := db.CreateResponse(before); err != nil {
		t.Fatal(err)
	}
	snap1, err := svc.PublishSnapshot(pid, "v1")
	if err != nil {
		t.Fatal(err)
	}
	after := &model.FrequencyResponse{
		ProjectID: pid, Kind: "after", BaseFreqHz: 442,
		Peaks: []model.Peak{{FreqHz: 884, Amplitude: 0.5}},
	}
	if _, err := db.CreateResponse(after); err != nil {
		t.Fatal(err)
	}
	current, err := db.BaseHashOfProject(pid)
	if err != nil {
		t.Fatal(err)
	}
	if current == snap1.BaseHash {
		t.Fatal("测量变更后基准哈希应变化")
	}
	snap2, err := svc.PublishSnapshot(pid, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if snap2.BaseHash != current {
		t.Fatalf("第二次发布应绑定最新基准: snap=%s current=%s", snap2.BaseHash, current)
	}
	if _, err := svc.SealProject(pid); err != nil {
		t.Fatalf("基准一致时应能封存: %v", err)
	}
}
