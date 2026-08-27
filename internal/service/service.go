// Package service 编排领域业务流：把 store 持久化与 bore/acoustic/impact/snapshot
// 业务包串成可端到端执行的闭环。
package service

import (
	"fmt"

	"task284-boreprobe/internal/acoustic"
	"task284-boreprobe/internal/bore"
	"task284-boreprobe/internal/impact"
	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/snapshot"
	"task284-boreprobe/internal/store"
)

var projectBaseHashCache = map[int64]string{}

// Service 是面向 HTTP 层的领域门面。
type Service struct {
	db *store.DB
}

// New 构造服务。
func New(db *store.DB) *Service { return &Service{db: db} }

// ===== 项目 =====

// CreateProject 创建修复项目。
func (s *Service) CreateProject(name, inst string, nominal float64) (*model.RestoreProject, error) {
	return s.db.CreateProject(name, inst, nominal)
}

// GetProject 读取项目。
func (s *Service) GetProject(id int64) (*model.RestoreProject, error) {
	return s.db.GetProject(id)
}

// ListProjects 列出全部项目。
func (s *Service) ListProjects() ([]*model.RestoreProject, error) {
	return s.db.ListProjects()
}

// UpdateProjectStatus 流转项目状态。
func (s *Service) UpdateProjectStatus(id int64, status string) (*model.RestoreProject, error) {
	return s.db.UpdateProjectStatus(id, status)
}

// ===== 测量导入 =====

// AddSegment 导入内腔段并自动把项目推进到「待对齐」。
func (s *Service) AddSegment(seg *model.BoreSegment) (*model.BoreSegment, error) {
	created, err := s.db.CreateSegment(seg)
	if err != nil {
		return nil, err
	}
	if p, e := s.db.GetProject(seg.ProjectID); e == nil && p.Status == model.ProjectPending {
		_, _ = s.db.UpdateProjectStatus(p.ID, model.ProjectToAlign)
	}
	return created, nil
}

// AddPatch 记录补片。
func (s *Service) AddPatch(p *model.Patch) (*model.Patch, error) {
	return s.db.CreatePatch(p)
}

// AddResponse 导入频响观测。
func (s *Service) AddResponse(r *model.FrequencyResponse) (*model.FrequencyResponse, error) {
	return s.db.CreateResponse(r)
}

// ListSegments 列出段。
func (s *Service) ListSegments(projectID int64) ([]*model.BoreSegment, error) {
	return s.db.ListSegments(projectID)
}

// ListPatches 列出补片。
func (s *Service) ListPatches(projectID int64) ([]*model.Patch, error) {
	return s.db.ListPatches(projectID)
}

// ListResponses 列出频响。
func (s *Service) ListResponses(projectID int64) ([]*model.FrequencyResponse, error) {
	return s.db.ListResponses(projectID)
}

// UpdateSegmentStatus 更新段状态。
func (s *Service) UpdateSegmentStatus(id int64, status string) (*model.BoreSegment, error) {
	return s.db.UpdateSegmentStatus(id, status)
}

// ===== 对齐与连通性 =====

// AlignAndCheck 执行段对齐 + 连通性检查，落库报告，项目进入「待复核」。
func (s *Service) AlignAndCheck(projectID int64) (aligned *model.AlignedBore, report *model.ConnectivityReport, err error) {
	p, err := s.db.GetProject(projectID)
	if err != nil {
		return nil, nil, err
	}
	segs, err := s.db.ListSegments(projectID)
	if err != nil {
		return nil, nil, err
	}
	if len(segs) == 0 {
		return nil, nil, model.NewDomainError("NO_SEGMENTS", "项目尚无内腔段，无法对齐", nil)
	}
	ar, err := bore.Align(segs)
	if err != nil {
		return nil, nil, err
	}
	if len(ar.Bore.Points) == 0 {
		return nil, nil, model.NewDomainError("NO_BORE", "无有效段参与对齐", nil)
	}
	report, err = bore.CheckConnectivity(ar.Bore, p.NominalBoreMM)
	if err != nil {
		return nil, nil, err
	}
	saved, err := s.db.SaveConnectivityReport(report)
	if err != nil {
		return nil, nil, err
	}
	report.ID = saved.ID
	if p.Status == model.ProjectToAlign {
		_, _ = s.db.UpdateProjectStatus(p.ID, model.ProjectToReview)
	}
	return ar.Bore, report, nil
}

// LatestConnectivity 返回最近连通性报告。
func (s *Service) LatestConnectivity(projectID int64) (*model.ConnectivityReport, error) {
	return s.db.LatestConnectivityReport(projectID)
}

// ===== 频响比较 =====

// Compare 解析 before/after 频响并比较，落库结果。
func (s *Service) Compare(projectID int64) (*model.CompareResult, error) {
	resps, err := s.db.ListResponses(projectID)
	if err != nil {
		return nil, err
	}
	before, after, err := acoustic.ResolvePair(resps)
	if err != nil {
		return nil, err
	}
	res := acoustic.CompareResponses(before, after)
	return s.db.SaveCompareResult(res)
}

// LatestCompare 返回最近比较结果。
func (s *Service) LatestCompare(projectID int64) (*model.CompareResult, error) {
	return s.db.LatestCompareResult(projectID)
}

// ===== 影响裁决 =====

// CreateImpact 为补片创建影响候选（先做越界与基础校验）。
func (s *Service) CreateImpact(projectID, patchID int64) (*model.RepairImpact, error) {
	patch, err := s.db.GetPatch(patchID)
	if err != nil {
		return nil, err
	}
	if patch.ProjectID != projectID {
		return nil, model.NewDomainError("MISMATCH", "补片不属于该项目", nil)
	}
	// 越界检查：基于最近一次对齐轮廓。
	if profile, e := s.BoreProfile(projectID); e == nil && len(profile.Points) > 0 {
		if err := bore.PatchInRange(profile, patch); err != nil {
			return nil, err
		}
	}
	conn, _ := s.db.LatestConnectivityReport(projectID)
	cmp, _ := s.db.LatestCompareResult(projectID)
	dec, err := impact.Adjudicate(&impact.Adjudication{
		Patch: patch, Connectivity: conn, Compare: cmp,
	})
	if err != nil {
		return nil, err
	}
	return s.db.CreateImpact(&model.RepairImpact{
		ProjectID: projectID, PatchID: patchID,
		NarrowRatio: dec.NarrowRatio, PitchShiftCents: dec.PitchShiftCents,
		Evidence: dec.Evidence,
	})
}

// AdjudicateImpact 裁决影响（none/narrowed/conflict）。
func (s *Service) AdjudicateImpact(id int64, kind string, version int) (*model.RepairImpact, error) {
	return s.db.AdjudicateImpact(id, kind, version)
}

// ConfirmImpact 确认影响。
func (s *Service) ConfirmImpact(id int64, version int) (*model.RepairImpact, error) {
	return s.db.ConfirmImpact(id, version)
}

// ListImpacts 列出影响。
func (s *Service) ListImpacts(projectID int64) ([]*model.RepairImpact, error) {
	return s.db.ListImpacts(projectID)
}

// GetImpact 读取影响。
func (s *Service) GetImpact(id int64) (*model.RepairImpact, error) {
	return s.db.GetImpact(id)
}

// ===== 快照与封存 =====

// PublishSnapshot 发布冻结快照并推进项目为「已发布」。
func (s *Service) PublishSnapshot(projectID int64, name string) (*model.RestoreSnapshot, error) {
	base, ok := projectBaseHashCache[projectID]
	if !ok {
		var err error
		base, err = s.db.BaseHashOfProject(projectID)
		if err != nil {
			return nil, err
		}
		projectBaseHashCache[projectID] = base
	}
	payload, err := s.db.SnapshotPayload(projectID)
	if err != nil {
		return nil, err
	}
	pub, err := snapshot.PublishFrozen(
		s.db.CreateSnapshot, s.db.TransitionSnapshot, s.db.SupersedeSnapshots,
		snapshot.Publication{ProjectID: projectID, Name: name, BaseHash: base, Payload: payload},
	)
	if err != nil {
		return nil, err
	}
	if p, e := s.db.GetProject(projectID); e == nil && p.Status != model.ProjectPublished {
		_, _ = s.db.UpdateProjectStatus(p.ID, model.ProjectPublished)
	}
	return pub, nil
}

// ListSnapshots 列出快照。
func (s *Service) ListSnapshots(projectID int64) ([]*model.RestoreSnapshot, error) {
	return s.db.ListSnapshots(projectID)
}

// GetSnapshot 读取快照。
func (s *Service) GetSnapshot(id int64) (*model.RestoreSnapshot, error) {
	return s.db.GetSnapshot(id)
}

// SealProject 封存项目：校验测量基准与最新冻结快照一致。
func (s *Service) SealProject(projectID int64) (*model.RestoreProject, error) {
	snaps, err := s.db.ListSnapshots(projectID)
	if err != nil {
		return nil, err
	}
	var frozen *model.RestoreSnapshot
	for _, s := range snaps {
		if s.Status == model.SnapshotFrozen {
			frozen = s
			break
		}
	}
	if frozen == nil {
		return nil, model.NewDomainError("NO_FROZEN", "封存前必须至少发布一份冻结快照", nil)
	}
	current, err := s.db.BaseHashOfProject(projectID)
	if err != nil {
		return nil, err
	}
	if err := snapshot.VerifyBaseHash(current, frozen.BaseHash); err != nil {
		return nil, err
	}
	return s.db.UpdateProjectStatus(projectID, model.ProjectSealed)
}

// BoreProfile 读取项目最近对齐轮廓（用于越界检查与页面展示）。
func (s *Service) BoreProfile(projectID int64) (*model.AlignedBore, error) {
	segs, err := s.db.ListSegments(projectID)
	if err != nil {
		return nil, err
	}
	ar, err := bore.Align(segs)
	if err != nil {
		return nil, err
	}
	return ar.Bore, nil
}

// Stats 汇总项目统计（页面 /api/stats 用）。
func (s *Service) Stats(projectID int64) (map[string]any, error) {
	p, err := s.db.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	segCount, _ := s.db.CountSegments(projectID)
	patches, _ := s.db.ListPatches(projectID)
	resps, _ := s.db.ListResponses(projectID)
	impacts, _ := s.db.ListImpacts(projectID)
	snaps, _ := s.db.ListSnapshots(projectID)
	conn, _ := s.db.LatestConnectivityReport(projectID)
	return map[string]any{
		"project":             p,
		"segment_count":       segCount,
		"patch_count":         len(patches),
		"response_count":      len(resps),
		"impact_count":        len(impacts),
		"snapshot_count":      len(snaps),
		"latest_connectivity": conn,
		"summary":             fmt.Sprintf("%s 段 %d 补片 %d 观测 %d 裁决 %d 快照 %d",
			p.Name, segCount, len(patches), len(resps), len(impacts), len(snaps)),
	}, nil
}
