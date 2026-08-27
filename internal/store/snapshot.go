package store

import (
	"database/sql"
	"fmt"
	"time"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// CreateSnapshot 创建修复快照（草稿）。冻结后任何修改被拒绝。
func (d *DB) CreateSnapshot(s *model.RestoreSnapshot) (*model.RestoreSnapshot, error) {
	if err := d.EnsureMutable(s.ProjectID); err != nil {
		return nil, err
	}
	if s.Name == "" || s.Payload == "" {
		return nil, model.NewDomainError("INVALID_INPUT", "快照名称与载荷必填", nil)
	}
	s.Status = model.SnapshotDraft
	s.CreatedAt = time.Now()
	res, err := d.Raw.Exec(
		`INSERT INTO snapshots(project_id, name, status, base_hash, payload, created_at)
		 VALUES(?,?,?,?,?,?)`,
		s.ProjectID, s.Name, s.Status, s.BaseHash, s.Payload,
		s.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "创建快照失败", err)
	}
	s.ID, _ = res.LastInsertId()
	return s, nil
}

// TransitionSnapshot 流转快照状态：draft→shared→frozen→superseded。
// 已冻结快照除被新快照替代外不可再变。
func (d *DB) TransitionSnapshot(id int64, target string) (*model.RestoreSnapshot, error) {
	s, err := d.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if !model.ValidSnapshotStatus(target) || target == model.SnapshotDraft {
		return nil, model.NewDomainError("INVALID_STATUS", "非法快照目标状态: "+target, nil)
	}
	if s.Status == model.SnapshotFrozen && target != model.SnapshotSuperseded {
		return nil, model.ErrFrozen
	}
	var frozenAt any
	if target == model.SnapshotFrozen {
		frozenAt = time.Now().Format(time.RFC3339Nano)
	}
	_, err = d.Raw.Exec(
		`UPDATE snapshots SET status=?, frozen_at=? WHERE id=?`,
		target, frozenAt, id)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "流转快照状态失败", err)
	}
	return d.GetSnapshot(id)
}

// GetSnapshot 按 ID 读取快照。
func (d *DB) GetSnapshot(id int64) (*model.RestoreSnapshot, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, name, status, base_hash, payload, created_at, frozen_at
		 FROM snapshots WHERE id=?`, id)
	return scanSnapshot(row)
}

func scanSnapshot(row *sql.Row) (*model.RestoreSnapshot, error) {
	var s model.RestoreSnapshot
	var createdAt string
	var frozenAt sql.NullString
	err := row.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Status, &s.BaseHash, &s.Payload, &createdAt, &frozenAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取快照失败", err)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		s.CreatedAt = t
	}
	if frozenAt.Valid {
		if t, e := time.Parse(time.RFC3339Nano, frozenAt.String); e == nil {
			s.FrozenAt = &t
		}
	}
	return &s, nil
}

// ListSnapshots 列出项目全部快照。
func (d *DB) ListSnapshots(projectID int64) ([]*model.RestoreSnapshot, error) {
	rows, err := d.Raw.Query(
		`SELECT id, project_id, name, status, base_hash, payload, created_at, frozen_at
		 FROM snapshots WHERE project_id=? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出快照失败", err)
	}
	defer rows.Close()
	var out []*model.RestoreSnapshot
	for rows.Next() {
		var s model.RestoreSnapshot
		var createdAt, frozenAt string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Status, &s.BaseHash, &s.Payload, &createdAt, &frozenAt); err != nil {
			return nil, err
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			s.CreatedAt = t
		}
		if frozenAt != "" {
			if t, e := time.Parse(time.RFC3339Nano, frozenAt); e == nil {
				s.FrozenAt = &t
			}
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// SupersedeSnapshots 把同一项目全部冻结/共享快照标记为替代（发布新冻结版时调用）。
func (d *DB) SupersedeSnapshots(projectID int64, keepID int64) error {
	return d.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE snapshots SET status='superseded'
			 WHERE project_id=? AND id<>? AND status IN ('frozen','shared')`,
			projectID, keepID)
		if err != nil {
			return model.Wrap("DB_ERROR", "替代旧快照失败", err)
		}
		if rows, _ := res.RowsAffected(); rows < 0 {
			return fmt.Errorf("supersede affected %d", rows)
		}
		return nil
	})
}

// BaseHashOfProject 汇总项目全部测量内容（段哈希 + 响应哈希 + 补片），
// 作为快照冻结时的测量基准哈希。
func (d *DB) BaseHashOfProject(projectID int64) (string, error) {
	segs, err := d.ListSegments(projectID)
	if err != nil {
		return "", err
	}
	resps, err := d.ListResponses(projectID)
	if err != nil {
		return "", err
	}
	patches, err := d.ListPatches(projectID)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, s := range segs {
		parts = append(parts, s.Hash)
	}
	for _, r := range resps {
		parts = append(parts, r.Hash)
	}
	for _, p := range patches {
		parts = append(parts, util.HashParts(
			timeFormat(p.CreatedAt), floatformat(p.AxialStart), floatformat(p.AxialEnd)))
	}
	return util.HashParts(parts...), nil
}

func timeFormat(t time.Time) string { return t.Format(time.RFC3339Nano) }
func floatformat(f float64) string  { return timeFormat(time.Unix(0, int64(f*1e9))) }
