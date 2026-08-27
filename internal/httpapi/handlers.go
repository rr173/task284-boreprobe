package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// decodeBody 解析请求体 JSON，失败统一返回 400。
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		util.Fail(w, http.StatusBadRequest, "BAD_BODY", "请求体 JSON 解析失败: "+err.Error())
		return false
	}
	return true
}

func (a *API) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string  `json:"name"`
		InstrumentType string  `json:"instrument_type"`
		NominalBoreMM  float64 `json:"nominal_bore_mm"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	p, err := a.svc.CreateProject(req.Name, req.InstrumentType, req.NominalBoreMM)
	if err != nil {
		handleError(w, err)
		return
	}
	util.Created(w, p)
}

func (a *API) handleListProjects(w http.ResponseWriter, r *http.Request) {
	list, err := a.svc.ListProjects()
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, list)
}

func (a *API) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	p, err := a.svc.GetProject(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, p)
}

func (a *API) handleProjectStatus(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	p, err := a.svc.UpdateProjectStatus(id, req.Status)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, p)
}

func (a *API) handleSealProject(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	p, err := a.svc.SealProject(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, p)
}

func (a *API) handleAddSegment(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var seg model.BoreSegment
	if !decodeBody(w, r, &seg) {
		return
	}
	seg.ProjectID = id
	created, err := a.svc.AddSegment(&seg)
	if err != nil {
		handleError(w, err)
		return
	}
	util.Created(w, created)
}

func (a *API) handleListSegments(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	list, err := a.svc.ListSegments(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, list)
}

func (a *API) handleUpdateSegment(w http.ResponseWriter, r *http.Request) {
	sid, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	seg, err := a.svc.UpdateSegmentStatus(sid, req.Status)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, seg)
}

func (a *API) handleAddPatch(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var p model.Patch
	if !decodeBody(w, r, &p) {
		return
	}
	p.ProjectID = id
	created, err := a.svc.AddPatch(&p)
	if err != nil {
		handleError(w, err)
		return
	}
	util.Created(w, created)
}

func (a *API) handleListPatches(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	list, err := a.svc.ListPatches(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, list)
}

func (a *API) handleAddResponse(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var resp model.FrequencyResponse
	if !decodeBody(w, r, &resp) {
		return
	}
	resp.ProjectID = id
	created, err := a.svc.AddResponse(&resp)
	if err != nil {
		handleError(w, err)
		return
	}
	util.Created(w, created)
}

func (a *API) handleListResponses(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	list, err := a.svc.ListResponses(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, list)
}

// ===== 对齐 / 连通性 / 比较 =====

func (a *API) handleAlign(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	bore, report, err := a.svc.AlignAndCheck(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, map[string]any{"aligned_bore": bore, "connectivity": report})
}

func (a *API) handleBoreProfile(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	bore, err := a.svc.BoreProfile(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, bore)
}

func (a *API) handleConnectivity(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	report, err := a.svc.LatestConnectivity(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, report)
}

func (a *API) handleCompare(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	res, err := a.svc.Compare(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, res)
}

func (a *API) handleGetCompare(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	res, err := a.svc.LatestCompare(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, res)
}

// ===== 影响 =====

func (a *API) handleCreateImpact(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var req struct {
		PatchID int64 `json:"patch_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	imp, err := a.svc.CreateImpact(id, req.PatchID)
	if err != nil {
		handleError(w, err)
		return
	}
	util.Created(w, imp)
}

func (a *API) handleListImpacts(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	list, err := a.svc.ListImpacts(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, list)
}

func (a *API) handleAdjudicateImpact(w http.ResponseWriter, r *http.Request) {
	iid, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var req struct {
		Kind    string `json:"kind"`
		Version int    `json:"version"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	imp, err := a.svc.AdjudicateImpact(iid, req.Kind, req.Version)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, imp)
}

func (a *API) handleConfirmImpact(w http.ResponseWriter, r *http.Request) {
	iid, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var req struct {
		Version int `json:"version"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	imp, err := a.svc.ConfirmImpact(iid, req.Version)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, imp)
}

// ===== 快照 / 统计 =====

func (a *API) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	snap, err := a.svc.PublishSnapshot(id, req.Name)
	if err != nil {
		handleError(w, err)
		return
	}
	util.Created(w, snap)
}

func (a *API) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	list, err := a.svc.ListSnapshots(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, list)
}

func (a *API) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	sid, err := util.ParseID(r.PathValue("sid"))
	if err != nil {
		handleError(w, err)
		return
	}
	snap, err := a.svc.GetSnapshot(sid)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, snap)
}

func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	id, err := util.ParseID(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	stats, err := a.svc.Stats(id)
	if err != nil {
		handleError(w, err)
		return
	}
	util.OK(w, stats)
}

// parseInt64 辅助（保留给后续扩展）。
func parseInt64(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}
