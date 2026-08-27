package store

import (
	"database/sql"
	"time"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// SaveConnectivityReport 保存连通性检查结果。
func (d *DB) SaveConnectivityReport(r *model.ConnectivityReport) (*model.ConnectivityReport, error) {
	if err := d.EnsureMutable(r.ProjectID); err != nil {
		return nil, err
	}
	if r.NominalDiameter <= 0 || r.MinDiameterMM < 0 {
		return nil, model.NewDomainError("INVALID_INPUT", "连通性结果缺少有效直径数据", nil)
	}
	gapsJSON, err := util.EncodeJSON(r.Gaps)
	if err != nil {
		return nil, model.Wrap("INVALID_INPUT", "断点编码失败", err)
	}
	r.Status = "open"
	r.CreatedAt = time.Now()
	res, err := d.Raw.Exec(
		`INSERT INTO connectivity_reports(project_id, min_diameter_mm, nominal_diameter_mm, narrow_ratio, gaps, status, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		r.ProjectID, r.MinDiameterMM, r.NominalDiameter, r.NarrowRatio, gapsJSON, r.Status,
		r.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "保存连通性报告失败", err)
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

// GetConnectivityReport 按 ID 读取连通性报告。
func (d *DB) GetConnectivityReport(id int64) (*model.ConnectivityReport, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, min_diameter_mm, nominal_diameter_mm, narrow_ratio, gaps, status, created_at
		 FROM connectivity_reports WHERE id=?`, id)
	return scanConnectivity(row)
}

func scanConnectivity(row *sql.Row) (*model.ConnectivityReport, error) {
	var r model.ConnectivityReport
	var gapsJSON, createdAt string
	err := row.Scan(&r.ID, &r.ProjectID, &r.MinDiameterMM, &r.NominalDiameter, &r.NarrowRatio,
		&gapsJSON, &r.Status, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取连通性报告失败", err)
	}
	if e := util.DecodeJSON(gapsJSON, &r.Gaps); e != nil {
		return nil, model.Wrap("DB_ERROR", "断点解析失败", e)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		r.CreatedAt = t
	}
	return &r, nil
}

// LatestConnectivityReport 返回项目最近一次连通性报告。
func (d *DB) LatestConnectivityReport(projectID int64) (*model.ConnectivityReport, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, min_diameter_mm, nominal_diameter_mm, narrow_ratio, gaps, status, created_at
		 FROM connectivity_reports WHERE project_id=? ORDER BY id DESC LIMIT 1`, projectID)
	return scanConnectivity(row)
}

// ListConnectivityReports 列出项目连通性报告。
func (d *DB) ListConnectivityReports(projectID int64) ([]*model.ConnectivityReport, error) {
	rows, err := d.Raw.Query(
		`SELECT id, project_id, min_diameter_mm, nominal_diameter_mm, narrow_ratio, gaps, status, created_at
		 FROM connectivity_reports WHERE project_id=? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出连通性报告失败", err)
	}
	defer rows.Close()
	var out []*model.ConnectivityReport
	for rows.Next() {
		var r model.ConnectivityReport
		var gapsJSON, createdAt string
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.MinDiameterMM, &r.NominalDiameter, &r.NarrowRatio,
			&gapsJSON, &r.Status, &createdAt); err != nil {
			return nil, err
		}
		if e := util.DecodeJSON(gapsJSON, &r.Gaps); e != nil {
			return nil, e
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			r.CreatedAt = t
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}
