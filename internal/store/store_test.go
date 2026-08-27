package store

import (
	"testing"

	"task284-boreprobe/internal/model"
)

func setupDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestProjectLifecycle(t *testing.T) {
	db := setupDB(t)
	p, err := db.CreateProject("测试项目", "oboe", 14.5)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != model.ProjectPending {
		t.Fatalf("初始状态应为 pending，实际 %s", p.Status)
	}
	got, err := db.GetProject(p.ID)
	if err != nil || got.Name != "测试项目" {
		t.Fatalf("读取项目失败: %v %v", got, err)
	}
	// 状态流转。
	if _, err := db.UpdateProjectStatus(p.ID, model.ProjectToAlign); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateProjectStatus(p.ID, model.ProjectSealed); err != nil {
		t.Fatal(err)
	}
	// 封存后拒绝流转。
	if _, err := db.UpdateProjectStatus(p.ID, model.ProjectToReview); err == nil {
		t.Fatal("封存后应拒绝状态流转")
	}
}

func TestSegmentIdempotent(t *testing.T) {
	db := setupDB(t)
	p, _ := db.CreateProject("项目", "clarinet", 14.0)
	seg := &model.BoreSegment{
		ProjectID: p.ID, Label: "段A", AxialStart: 0, AxialEnd: 10,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{14, 14, 14, 14, 14},
	}
	s1, err := db.CreateSegment(seg)
	if err != nil {
		t.Fatal(err)
	}
	// 相同内容重复导入 → 返回同一记录（幂等）。
	s2, err := db.CreateSegment(seg)
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID != s2.ID {
		t.Fatalf("幂等失败: %d != %d", s1.ID, s2.ID)
	}
}

func TestImpactVersionConflict(t *testing.T) {
	db := setupDB(t)
	p, _ := db.CreateProject("项目", "recorder", 14.0)
	patch, err := db.CreatePatch(&model.Patch{
		ProjectID: p.ID, AxialStart: 10, AxialEnd: 20, ThicknessMM: 1, Material: "枫木",
	})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := db.CreateImpact(&model.RepairImpact{ProjectID: p.ID, PatchID: patch.ID})
	if err != nil {
		t.Fatal(err)
	}
	// 第一次裁决成功。
	if _, err := db.AdjudicateImpact(imp.ID, model.ImpactNone, imp.Version); err != nil {
		t.Fatal(err)
	}
	// 用过期版本再裁决 → ErrVersionStale。
	if _, err := db.AdjudicateImpact(imp.ID, model.ImpactConflict, imp.Version); err == nil {
		t.Fatal("过期版本应触发 ErrVersionStale")
	}
}

func TestSnapshotFrozenLock(t *testing.T) {
	db := setupDB(t)
	p, _ := db.CreateProject("项目", "oboe", 14.5)
	snap, err := db.CreateSnapshot(&model.RestoreSnapshot{
		ProjectID: p.ID, Name: "v1", Payload: `{"x":1}`, BaseHash: "abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.TransitionSnapshot(snap.ID, model.SnapshotFrozen); err != nil {
		t.Fatal(err)
	}
	// 冻结后不允许再共享。
	if _, err := db.TransitionSnapshot(snap.ID, model.SnapshotShared); err == nil {
		t.Fatal("冻结后应拒绝共享流转")
	}
	// 允许被替代。
	if _, err := db.TransitionSnapshot(snap.ID, model.SnapshotSuperseded); err != nil {
		t.Fatalf("冻结快照应可被替代: %v", err)
	}
}

func TestBaseHashStable(t *testing.T) {
	db := setupDB(t)
	p, _ := db.CreateProject("项目", "oboe", 14.5)
	seg := &model.BoreSegment{
		ProjectID: p.ID, Label: "A", AxialStart: 0, AxialEnd: 10,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{14.5, 14.5, 14.4, 14.5, 14.5},
	}
	if _, err := db.CreateSegment(seg); err != nil {
		t.Fatal(err)
	}
	h1, err := db.BaseHashOfProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := db.BaseHashOfProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("测量基准哈希不稳定: %s != %s", h1, h2)
	}
}
