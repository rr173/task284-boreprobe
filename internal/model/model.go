// Package model 定义历史木管乐器内腔修复证据复核台的领域实体与状态机。
package model

import (
	"time"
)

// 项目状态机：整理中 → 待对齐 → 待复核 → 已发布 → 封存。
// 封存后任何测量、裁决与快照修改均被拒绝。
const (
	ProjectPending   = "pending"   // 整理中：等待导入测量
	ProjectToAlign   = "to_align"  // 待对齐：段已导入，等待执行对齐
	ProjectToReview  = "to_review" // 待复核：对齐与响应比较完成，等待裁决
	ProjectPublished = "published" // 已发布：至少一份快照已冻结
	ProjectSealed    = "sealed"    // 封存：测量基准已冻结，不可再变更
)

// 内腔段状态机：原始 → 已对齐 | 缺失 | 异常 → 排除。
const (
	SegmentRaw      = "raw"      // 原始：刚导入的测量段
	SegmentAligned  = "aligned"  // 已对齐：通过尺度校正并入连续轮廓
	SegmentMissing  = "missing"  // 缺失：扫描不完整，标记等待补测
	SegmentAnomalous = "anomalous" // 异常：尺度单位不明或内径矛盾
	SegmentExcluded = "excluded" // 排除：不参与连通性与响应复核
)

// 修复影响状态机：候选 → 无影响 | 缩窄 | 响应冲突 → 确认。
const (
	ImpactCandidate = "candidate" // 候选：待复核的修复影响
	ImpactNone      = "none"      // 无影响：未检测到缩窄与音高偏移
	ImpactNarrowed  = "narrowed"  // 缩窄：补片区域内径显著低于标称
	ImpactConflict  = "conflict"  // 响应冲突：修复前后频响残差显著
	ImpactConfirmed = "confirmed" // 确认：研究者采纳该裁决
)

// 修复快照状态机：草稿 → 共享 → 冻结 → 替代。
const (
	SnapshotDraft      = "draft"      // 草稿：尚未对外可见
	SnapshotShared     = "shared"     // 共享：可被复核人查看
	SnapshotFrozen     = "frozen"     // 冻结：绑定测量基准，不可修改
	SnapshotSuperseded = "superseded" // 替代：被新快照取代
)

// RestoreProject 是一次历史木管乐器内腔修复的复核单元。
type RestoreProject struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	InstrumentType string    `json:"instrument_type"` // 如 oboe / clarinet / recorder
	NominalBoreMM  float64   `json:"nominal_bore_mm"` // 标称内径，缩窄判定的基准
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	SealedAt       *time.Time `json:"sealed_at,omitempty"`
}

// BoreSegment 是一段内腔扫描测量，轴向区间为 [AxialStart, AxialEnd]。
// DiameterMM 与采样点一一对应，PitchMM 为相邻采样点间距。
type BoreSegment struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	Label       string    `json:"label"`
	AxialStart  float64   `json:"axial_start"` // 轴向起点（mm，统一单位后）
	AxialEnd    float64   `json:"axial_end"`   // 轴向终点（mm）
	DiameterMM  []float64 `json:"diameter_mm"` // 内径序列
	PitchMM     float64   `json:"pitch_mm"`    // 采样间距
	Unit        string    `json:"unit"`        // 导入时单位：mm / cm，对齐时统一
	Status      string    `json:"status"`
	Hash        string    `json:"hash"` // 幂等摘要，防止重复导入
	CreatedAt   time.Time `json:"created_at"`
}

// Patch 是一块修复补片的测量记录：覆盖轴向区间并给出厚度与材料。
type Patch struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	AxialStart  float64   `json:"axial_start"`
	AxialEnd    float64   `json:"axial_end"`
	ThicknessMM float64   `json:"thickness_mm"`
	Material    string    `json:"material"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

// Peak 是频响中的一个谐振峰。
type Peak struct {
	FreqHz    float64 `json:"freq_hz"`
	Amplitude float64 `json:"amplitude"`
}

// FrequencyResponse 是一次频响观测：修复前（before）或修复后（after）。
type FrequencyResponse struct {
	ID         int64     `json:"id"`
	ProjectID  int64     `json:"project_id"`
	Kind       string    `json:"kind"` // before / after
	BaseFreqHz float64   `json:"base_freq_hz"`
	Peaks      []Peak    `json:"peaks"`
	CaptureAt  string    `json:"capture_at"`
	Hash       string    `json:"hash"`
	CreatedAt  time.Time `json:"created_at"`
}

// Gap 是连通性检查发现的管腔断点或数据缺口。
type Gap struct {
	AxialStart float64 `json:"axial_start"`
	AxialEnd   float64 `json:"axial_end"`
	Reason     string  `json:"reason"` // missing_scan / diameter_drop / scale_mismatch
}

// ConnectivityReport 是内腔连通性检查的结果。
type ConnectivityReport struct {
	ID              int64     `json:"id"`
	ProjectID       int64     `json:"project_id"`
	MinDiameterMM   float64   `json:"min_diameter_mm"`
	NominalDiameter float64   `json:"nominal_diameter_mm"`
	NarrowRatio     float64   `json:"narrow_ratio"` // min / nominal，<1 表示缩窄
	Gaps            []Gap     `json:"gaps"`
	Status          string    `json:"status"` // open / reviewed
	CreatedAt       time.Time `json:"created_at"`
}

// RepairImpact 是对一块补片修复影响的裁决。
type RepairImpact struct {
	ID              int64     `json:"id"`
	ProjectID       int64     `json:"project_id"`
	PatchID         int64     `json:"patch_id"`
	Kind            string    `json:"kind"`
	NarrowRatio     float64   `json:"narrow_ratio"`
	PitchShiftCents float64   `json:"pitch_shift_cents"`
	Evidence        string    `json:"evidence"` // 裁决依据摘要
	Version         int       `json:"version"`  // 乐观锁版本
	CreatedAt       time.Time `json:"created_at"`
}

// RestoreSnapshot 是一次修复证据版本发布。
type RestoreSnapshot struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	BaseHash    string    `json:"base_hash"` // 测量基准哈希，冻结时固化
	Payload     string    `json:"payload"`   // 快照载荷（剖面+裁决 JSON）
	CreatedAt   time.Time `json:"created_at"`
	FrozenAt    *time.Time `json:"frozen_at,omitempty"`
}

// BorePoint 是对齐后连续内腔轮廓上的一个采样点。
type BorePoint struct {
	AxialMM     float64 `json:"axial_mm"`
	DiameterMM  float64 `json:"diameter_mm"`
	SegmentID   int64   `json:"segment_id"`
	SegmentLabel string  `json:"segment_label"`
}

// AlignedBore 是尺度校正后拼接而成的连续内腔轮廓。
type AlignedBore struct {
	ProjectID  int64       `json:"project_id"`
	Points     []BorePoint `json:"points"`
	SegmentIDs []int64     `json:"segment_ids"`
	ScaleNote  string      `json:"scale_note"`
}

// CompareResult 是修复前后频响比较的结果。
type CompareResult struct {
	ID              int64   `json:"id"`
	ProjectID       int64   `json:"project_id"`
	BeforeID        int64   `json:"before_id"`
	AfterID         int64   `json:"after_id"`
	PitchShiftCents float64 `json:"pitch_shift_cents"`
	ResidualRMS     float64 `json:"residual_rms"`
	Significant     bool    `json:"significant"`
	Detail          string  `json:"detail"`
	CreatedAt       time.Time `json:"created_at"`
}
