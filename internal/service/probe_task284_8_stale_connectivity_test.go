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

func TestCreateImpactUsesLatestConnectivity(t *testing.T) {
	svc, db := newTestService(t)
	pid := mustProject(t, db)
	seg1 := &model.BoreSegment{
		ProjectID: pid, Label: "宽段", AxialStart: 0, AxialEnd: 10,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{14.5, 14.5, 14.5, 14.5, 14.5},
	}
	if _, err := db.CreateSegment(seg1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AlignAndCheck(pid); err != nil {
		t.Fatal(err)
	}
	conn1, err := db.LatestConnectivityReport(pid)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := db.CreatePatch(&model.Patch{ProjectID: pid, AxialStart: 2, AxialEnd: 6, ThicknessMM: 1.0, Material: "枫木"})
	if err != nil {
		t.Fatal(err)
	}
	seg2 := &model.BoreSegment{
		ProjectID: pid, Label: "窄段", AxialStart: 12, AxialEnd: 22,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{11.0, 11.0, 11.0, 11.0, 11.0},
	}
	if _, err := db.CreateSegment(seg2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AlignAndCheck(pid); err != nil {
		t.Fatal(err)
	}
	conn2, err := db.LatestConnectivityReport(pid)
	if err != nil {
		t.Fatal(err)
	}
	if conn2.NarrowRatio >= conn1.NarrowRatio {
		t.Fatalf("重对齐后缩窄比应下降: before=%.3f after=%.3f", conn1.NarrowRatio, conn2.NarrowRatio)
	}
	imp, err := svc.CreateImpact(pid, patch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if imp.NarrowRatio != conn2.NarrowRatio {
		t.Fatalf("CreateImpact 使用了陈旧连通性: impact=%.3f latest=%.3f", imp.NarrowRatio, conn2.NarrowRatio)
	}
}
