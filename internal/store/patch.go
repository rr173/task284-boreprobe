package store

import (
	"database/sql"
	"time"

	"task284-boreprobe/internal/model"
)

// CreatePatch 记录一块修复补片。轴向区间必须为正。
func (d *DB) CreatePatch(p *model.Patch) (*model.Patch, error) {
	if err := d.EnsureMutable(p.ProjectID); err != nil {
		return nil, err
	}
	if p.AxialEnd <= p.AxialStart || p.ThicknessMM < 0 || p.Material == "" {
		return nil, model.NewDomainError("INVALID_INPUT", "补片区间、厚度与材料必填", nil)
	}
	p.CreatedAt = time.Now()
	res, err := d.Raw.Exec(
		`INSERT INTO patches(project_id, axial_start, axial_end, thickness_mm, material, note, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		p.ProjectID, p.AxialStart, p.AxialEnd, p.ThicknessMM, p.Material, p.Note,
		p.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "记录补片失败", err)
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

// GetPatch 按 ID 读取补片。
func (d *DB) GetPatch(id int64) (*model.Patch, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, axial_start, axial_end, thickness_mm, material, note, created_at
		 FROM patches WHERE id=?`, id)
	return scanPatch(row)
}

func scanPatch(row *sql.Row) (*model.Patch, error) {
	var p model.Patch
	var createdAt string
	err := row.Scan(&p.ID, &p.ProjectID, &p.AxialStart, &p.AxialEnd, &p.ThicknessMM,
		&p.Material, &p.Note, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取补片失败", err)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		p.CreatedAt = t
	}
	return &p, nil
}

// ListPatches 列出项目全部补片。
func (d *DB) ListPatches(projectID int64) ([]*model.Patch, error) {
	rows, err := d.Raw.Query(
		`SELECT id, project_id, axial_start, axial_end, thickness_mm, material, note, created_at
		 FROM patches WHERE project_id=? ORDER BY axial_start`, projectID)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出补片失败", err)
	}
	defer rows.Close()
	var out []*model.Patch
	for rows.Next() {
		var p model.Patch
		var createdAt string
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.AxialStart, &p.AxialEnd, &p.ThicknessMM,
			&p.Material, &p.Note, &createdAt); err != nil {
			return nil, err
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			p.CreatedAt = t
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}
