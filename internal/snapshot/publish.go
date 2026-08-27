// Package snapshot 实现修复快照的发布、冻结与替代。
// 冻结快照绑定测量基准哈希：封存后测量变更会被拒绝。
package snapshot

import (
	"time"

	"task284-boreprobe/internal/model"
)

// Publication 是一次快照发布的输入。
type Publication struct {
	ProjectID int64
	Name      string
	BaseHash  string
	Payload   string
}

// PublishFrozen 发布一份冻结快照：
// 1) 创建草稿快照；2) 把项目旧冻结/共享快照标记 superseded；
// 3) 冻结新快照（绑定测量基准哈希）。
// 返回发布后的快照实体。
func PublishFrozen(create func(*model.RestoreSnapshot) (*model.RestoreSnapshot, error),
	transition func(id int64, target string) (*model.RestoreSnapshot, error),
	supersede func(projectID int64, keepID int64) error,
	pub Publication) (*model.RestoreSnapshot, error) {
	if pub.Name == "" || pub.Payload == "" || pub.BaseHash == "" {
		return nil, model.NewDomainError("INVALID_INPUT", "快照名称、载荷与基准哈希必填", nil)
	}
	s, err := create(&model.RestoreSnapshot{
		ProjectID: pub.ProjectID,
		Name:      pub.Name,
		BaseHash:  pub.BaseHash,
		Payload:   pub.Payload,
	})
	if err != nil {
		return nil, err
	}
	if err := supersede(pub.ProjectID, s.ID); err != nil {
		return nil, err
	}
	return transition(s.ID, model.SnapshotFrozen)
}

// VerifyBaseHash 校验当前测量基准是否与快照冻结时一致。
// 用于「封存前核对」：不一致说明测量被改动，拒绝封存。
func VerifyBaseHash(current, frozen string) error {
	if current != frozen {
		return model.NewDomainError("BASE_HASH_MISMATCH",
			"当前测量基准与快照冻结基准不一致，请先发布新快照", nil)
	}
	return nil
}

// FrozenAt 返回冻结时间（调用方用于展示）。
func FrozenAt() string { return time.Now().Format(time.RFC3339Nano) }
