package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/service"
	"task284-boreprobe/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "probe-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(service.New(db)).Handler()
}

func TestStaleImpactVersionReturns409(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "probe-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.New(db)
	h := New(svc).Handler()
	p, err := db.CreateProject("版本冲突", "oboe", 14.5)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := db.CreatePatch(&model.Patch{ProjectID: p.ID, AxialStart: 10, AxialEnd: 20, ThicknessMM: 1, Material: "枫木"})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := db.CreateImpact(&model.RepairImpact{ProjectID: p.ID, PatchID: patch.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AdjudicateImpact(imp.ID, model.ImpactNone, imp.Version); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"kind":"conflict","version":1}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/impacts/1", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("过期版本应 409，实际 %d body=%s", rec.Code, rec.Body.String())
	}
}
