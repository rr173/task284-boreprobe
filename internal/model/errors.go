package model

import (
	"errors"
	"fmt"
)

// 领域错误：统一映射为 HTTP 4xx/409 响应。
var (
	ErrNotFound         = errors.New("资源不存在")
	ErrConflict         = errors.New("状态冲突")
	ErrSealed           = errors.New("项目已封存，拒绝修改")
	ErrFrozen           = errors.New("快照已冻结，拒绝修改")
	ErrPatchOutOfRange  = errors.New("补片轴向区间超出内腔段覆盖范围")
	ErrOverlapContradict = errors.New("内腔段轴向重叠但内径矛盾")
	ErrScaleUnknown     = errors.New("尺度单位不明或无法换算")
	ErrInvalidInput     = errors.New("输入数据非法")
	ErrDuplicate        = errors.New("重复导入，摘要已存在")
	ErrVersionStale     = errors.New("裁决版本过期，请刷新后重试")
)

// DomainError 携带面向调用方的可读消息与建议动作。
type DomainError struct {
	Code    string
	Message string
	Cause   error
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// NewDomainError 构造领域错误。
func NewDomainError(code, msg string, cause error) *DomainError {
	return &DomainError{Code: code, Message: msg, Cause: cause}
}

// Wrap 把底层错误包装为领域错误。
func Wrap(code, msg string, err error) error {
	if err == nil {
		return nil
	}
	if de, ok := err.(*DomainError); ok {
		return de
	}
	return NewDomainError(code, msg, err)
}

// ValidProjectStatus 校验项目状态是否合法。
func ValidProjectStatus(s string) bool {
	switch s {
	case ProjectPending, ProjectToAlign, ProjectToReview, ProjectPublished, ProjectSealed:
		return true
	}
	return false
}

// ValidSegmentStatus 校验内腔段状态是否合法。
func ValidSegmentStatus(s string) bool {
	switch s {
	case SegmentRaw, SegmentAligned, SegmentMissing, SegmentAnomalous, SegmentExcluded:
		return true
	}
	return false
}

// ValidImpactKind 校验修复影响类型是否合法。
func ValidImpactKind(k string) bool {
	switch k {
	case ImpactCandidate, ImpactNone, ImpactNarrowed, ImpactConflict, ImpactConfirmed:
		return true
	}
	return false
}

// ValidSnapshotStatus 校验快照状态是否合法。
func ValidSnapshotStatus(s string) bool {
	switch s {
	case SnapshotDraft, SnapshotShared, SnapshotFrozen, SnapshotSuperseded:
		return true
	}
	return false
}

// ValidResponseKind 校验频响观测类型是否合法。
func ValidResponseKind(k string) bool {
	return k == "before" || k == "after"
}
