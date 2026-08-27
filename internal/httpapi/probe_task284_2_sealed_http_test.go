package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

func TestSealedProjectSegmentReturns409(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"name":"封存测试","instrument_type":"oboe","nominal_bore_mm":14.5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/projects", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project=%d body=%s", rec.Code, rec.Body.String())
	}
	segBody := strings.NewReader(`{"label":"段A","axial_start":0,"axial_end":10,"pitch_mm":2,"unit":"mm","diameter_mm":[14,14,14]}`)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/1/segments", segBody)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed segment=%d body=%s", rec.Code, rec.Body.String())
	}
	snapBody := strings.NewReader(`{"name":"v1"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/1/snapshots", snapBody)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish snapshot=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/projects/1/seal", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seal=%d body=%s", rec.Code, rec.Body.String())
	}
	segBody = strings.NewReader(`{"label":"段B","axial_start":20,"axial_end":30,"pitch_mm":2,"unit":"mm","diameter_mm":[14,14,14]}`)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/1/segments", segBody)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("封存后导入段应 409，实际 %d body=%s", rec.Code, rec.Body.String())
	}
}
