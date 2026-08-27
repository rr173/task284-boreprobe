// Package httpapi 提供 HTTP 层：路由注册、请求解析与统一错误映射。
package httpapi

import (
	"errors"
	"log"
	"net/http"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/service"
	"task284-boreprobe/internal/util"
)

// API 持有服务依赖与路由器。
type API struct {
	svc *service.Service
	mux *http.ServeMux
}

// New 构造 API 并注册全部路由（Go 1.22+ 方法路由）。
func New(svc *service.Service) *API {
	a := &API{svc: svc, mux: http.NewServeMux()}
	a.routes()
	return a
}

// Handler 返回可挂载的 http.Handler。
func (a *API) Handler() http.Handler { return a.mux }

func (a *API) routes() {
	// 页面
	a.mux.HandleFunc("GET /{$}", a.handleIndex)

	// 健康
	a.mux.HandleFunc("GET /api/health", a.handleHealth)

	// 项目
	a.mux.HandleFunc("POST /api/projects", a.handleCreateProject)
	a.mux.HandleFunc("GET /api/projects", a.handleListProjects)
	a.mux.HandleFunc("GET /api/projects/{id}", a.handleGetProject)
	a.mux.HandleFunc("PATCH /api/projects/{id}/status", a.handleProjectStatus)
	a.mux.HandleFunc("POST /api/projects/{id}/seal", a.handleSealProject)

	// 内腔段
	a.mux.HandleFunc("POST /api/projects/{id}/segments", a.handleAddSegment)
	a.mux.HandleFunc("GET /api/projects/{id}/segments", a.handleListSegments)
	a.mux.HandleFunc("PATCH /api/segments/{id}", a.handleUpdateSegment)

	// 补片
	a.mux.HandleFunc("POST /api/projects/{id}/patches", a.handleAddPatch)
	a.mux.HandleFunc("GET /api/projects/{id}/patches", a.handleListPatches)

	// 频响
	a.mux.HandleFunc("POST /api/projects/{id}/responses", a.handleAddResponse)
	a.mux.HandleFunc("GET /api/projects/{id}/responses", a.handleListResponses)

	// 对齐与连通性
	a.mux.HandleFunc("POST /api/projects/{id}/align", a.handleAlign)
	a.mux.HandleFunc("GET /api/projects/{id}/bore/profile", a.handleBoreProfile)
	a.mux.HandleFunc("GET /api/projects/{id}/connectivity", a.handleConnectivity)

	// 频响比较
	a.mux.HandleFunc("POST /api/projects/{id}/compare", a.handleCompare)
	a.mux.HandleFunc("GET /api/projects/{id}/compare", a.handleGetCompare)

	// 影响裁决
	a.mux.HandleFunc("POST /api/projects/{id}/impacts", a.handleCreateImpact)
	a.mux.HandleFunc("GET /api/projects/{id}/impacts", a.handleListImpacts)
	a.mux.HandleFunc("PATCH /api/impacts/{id}", a.handleAdjudicateImpact)
	a.mux.HandleFunc("POST /api/impacts/{id}/confirm", a.handleConfirmImpact)

	// 快照
	a.mux.HandleFunc("POST /api/projects/{id}/snapshots", a.handlePublishSnapshot)
	a.mux.HandleFunc("GET /api/projects/{id}/snapshots", a.handleListSnapshots)
	a.mux.HandleFunc("GET /api/projects/{id}/snapshots/{sid}", a.handleGetSnapshot)

	// 统计
	a.mux.HandleFunc("GET /api/projects/{id}/stats", a.handleStats)
}

// handleError 把领域错误映射为 HTTP 状态码。
func handleError(w http.ResponseWriter, err error) {
	switch err {
	case model.ErrNotFound:
		util.Fail(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case model.ErrSealed:
		util.Fail(w, http.StatusConflict, "SEALED", err.Error())
	case model.ErrFrozen:
		util.Fail(w, http.StatusConflict, "FROZEN", err.Error())
	case model.ErrVersionStale:
		util.Fail(w, http.StatusConflict, "VERSION_STALE", err.Error())
	case model.ErrPatchOutOfRange:
		util.Fail(w, http.StatusUnprocessableEntity, "PATCH_OUT_OF_RANGE", err.Error())
	case model.ErrOverlapContradict:
		util.Fail(w, http.StatusUnprocessableEntity, "OVERLAP_CONTRADICT", err.Error())
	case model.ErrScaleUnknown:
		util.Fail(w, http.StatusUnprocessableEntity, "SCALE_UNKNOWN", err.Error())
	case model.ErrDuplicate:
		util.Fail(w, http.StatusConflict, "DUPLICATE", err.Error())
	default:
		var de *model.DomainError
		if errors.As(err, &de) {
			status := http.StatusBadRequest
			if de.Code == "DB_ERROR" {
				status = http.StatusInternalServerError
			}
			util.Fail(w, status, de.Code, de.Message)
			log.Printf("domain error %s: %v", de.Code, err)
			return
		}
		log.Printf("internal error: %v", err)
		util.Fail(w, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
	}
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	util.OK(w, map[string]any{"status": "ok", "service": "task284-boreprobe"})
}
