package store

import (
	"database/sql"
	"time"

	"task284-boreprobe/internal/model"
)

// CreateImpact 创建修复影响候选（version=1）。
func (d *DB) CreateImpact(imp *model.RepairImpact) (*model.RepairImpact, error) {
	if err := d.EnsureMutable(imp.ProjectID); err != nil {
		return nil, err
	}
	if imp.PatchID <= 0 {
		return nil, model.NewDomainError("INVALID_INPUT", "影响候选必须关联补片", nil)
	}
	if _, err := d.GetPatch(imp.PatchID); err != nil {
		return nil, err
	}
	imp.Kind = model.ImpactCandidate
	imp.Version = 1
	imp.CreatedAt = time.Now()
	res, err := d.Raw.Exec(
		`INSERT INTO impacts(project_id, patch_id, kind, narrow_ratio, pitch_shift_cents, evidence, version, created_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		imp.ProjectID, imp.PatchID, imp.Kind, imp.NarrowRatio, imp.PitchShiftCents,
		imp.Evidence, imp.Version, imp.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "创建影响候选失败", err)
	}
	imp.ID, _ = res.LastInsertId()
	return imp, nil
}

// AdjudicateImpact 裁决影响：乐观锁版本校验，版本不符返回 ErrVersionStale。
// 同一项目封存后拒绝任何裁决。
func (d *DB) AdjudicateImpact(id int64, kind string, expectedVersion int) (*model.RepairImpact, error) {
	imp, err := d.GetImpact(id)
	if err != nil {
		return nil, err
	}
	if err := d.EnsureMutable(imp.ProjectID); err != nil {
		return nil, err
	}
	if !model.ValidImpactKind(kind) || kind == model.ImpactCandidate || kind == model.ImpactConfirmed {
		return nil, model.NewDomainError("INVALID_STATUS", "裁决结果须为 none/narrowed/conflict", nil)
	}
	if expectedVersion != imp.Version {
		return nil, model.ErrVersionStale
	}
	time.Sleep(10 * time.Millisecond)
	updated := time.Now().Format(time.RFC3339Nano)
	// 裁决并发安全：UPDATE 时校验版本，影响行数为 0 即版本已被抢占。
	res, err := d.Raw.Exec(
		`UPDATE impacts SET kind=?, evidence=?, version=version+1 WHERE id=?`,
		kind, imp.Evidence, id)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "裁决影响失败", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, model.ErrVersionStale
	}
	_ = updated
	return d.GetImpact(id)
}

// ConfirmImpact 研究者确认裁决（candidate/none/narrowed/conflict → confirmed）。
func (d *DB) ConfirmImpact(id int64, expectedVersion int) (*model.RepairImpact, error) {
	imp, err := d.GetImpact(id)
	if err != nil {
		return nil, err
	}
	if err := d.EnsureMutable(imp.ProjectID); err != nil {
		return nil, err
	}
	if imp.Kind != model.ImpactNone && imp.Kind != model.ImpactNarrowed && imp.Kind != model.ImpactConflict {
		return nil, model.NewDomainError("INVALID_STATUS", "仅已裁决的影响可确认", nil)
	}
	if expectedVersion != imp.Version {
		return nil, model.ErrVersionStale
	}
	res, err := d.Raw.Exec(
		`UPDATE impacts SET kind='confirmed', version=version+1 WHERE id=? AND version=?`,
		id, expectedVersion)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "确认影响失败", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, model.ErrVersionStale
	}
	return d.GetImpact(id)
}

// GetImpact 按 ID 读取影响。
func (d *DB) GetImpact(id int64) (*model.RepairImpact, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, patch_id, kind, narrow_ratio, pitch_shift_cents, evidence, version, created_at
		 FROM impacts WHERE id=?`, id)
	return scanImpact(row)
}

func scanImpact(row *sql.Row) (*model.RepairImpact, error) {
	var imp model.RepairImpact
	var createdAt string
	err := row.Scan(&imp.ID, &imp.ProjectID, &imp.PatchID, &imp.Kind, &imp.NarrowRatio,
		&imp.PitchShiftCents, &imp.Evidence, &imp.Version, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取影响失败", err)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		imp.CreatedAt = t
	}
	return &imp, nil
}

// ListImpacts 列出项目全部影响。
func (d *DB) ListImpacts(projectID int64) ([]*model.RepairImpact, error) {
	rows, err := d.Raw.Query(
		`SELECT id, project_id, patch_id, kind, narrow_ratio, pitch_shift_cents, evidence, version, created_at
		 FROM impacts WHERE project_id=? ORDER BY id`, projectID)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出影响失败", err)
	}
	defer rows.Close()
	var out []*model.RepairImpact
	for rows.Next() {
		var imp model.RepairImpact
		var createdAt string
		if err := rows.Scan(&imp.ID, &imp.ProjectID, &imp.PatchID, &imp.Kind, &imp.NarrowRatio,
			&imp.PitchShiftCents, &imp.Evidence, &imp.Version, &createdAt); err != nil {
			return nil, err
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			imp.CreatedAt = t
		}
		out = append(out, &imp)
	}
	return out, rows.Err()
}
