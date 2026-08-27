package store

import (
	"database/sql"
	"time"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// SaveCompareResult 保存一次频响比较结果。
func (d *DB) SaveCompareResult(r *model.CompareResult) (*model.CompareResult, error) {
	if err := d.EnsureMutable(r.ProjectID); err != nil {
		return nil, err
	}
	sig := 0
	if r.Significant {
		sig = 1
	}
	r.CreatedAt = time.Now()
	res, err := d.Raw.Exec(
		`INSERT INTO compare_results(project_id, before_id, after_id, pitch_shift_cents, residual_rms, significant, detail, created_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		r.ProjectID, r.BeforeID, r.AfterID, r.PitchShiftCents, r.ResidualRMS, sig, r.Detail,
		r.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "保存比较结果失败", err)
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

// GetCompareResult 按 ID 读取比较结果。
func (d *DB) GetCompareResult(id int64) (*model.CompareResult, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, before_id, after_id, pitch_shift_cents, residual_rms, significant, detail, created_at
		 FROM compare_results WHERE id=?`, id)
	return scanCompare(row)
}

func scanCompare(row *sql.Row) (*model.CompareResult, error) {
	var r model.CompareResult
	var sig int
	var createdAt string
	err := row.Scan(&r.ID, &r.ProjectID, &r.BeforeID, &r.AfterID, &r.PitchShiftCents,
		&r.ResidualRMS, &sig, &r.Detail, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取比较结果失败", err)
	}
	r.Significant = sig == 1
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		r.CreatedAt = t
	}
	return &r, nil
}

// LatestCompareResult 返回项目最近一次比较结果。
func (d *DB) LatestCompareResult(projectID int64) (*model.CompareResult, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, before_id, after_id, pitch_shift_cents, residual_rms, significant, detail, created_at
		 FROM compare_results WHERE project_id=? ORDER BY id DESC LIMIT 1`, projectID)
	return scanCompare(row)
}

// SnapshotPayload 组装快照载荷：把项目全量测量与裁决写入 JSON。
func (d *DB) SnapshotPayload(projectID int64) (string, error) {
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
	impacts, err := d.ListImpacts(projectID)
	if err != nil {
		return "", err
	}
	conn, err := d.LatestConnectivityReport(projectID)
	if err != nil && err != model.ErrNotFound {
		return "", err
	}
	cmp, err := d.LatestCompareResult(projectID)
	if err != nil && err != model.ErrNotFound {
		return "", err
	}
	payload := map[string]any{
		"segments":    segs,
		"responses":   resps,
		"patches":     patches,
		"impacts":     impacts,
		"connectivity": conn,
		"compare":     cmp,
	}
	return util.EncodeJSON(payload)
}
