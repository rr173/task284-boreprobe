package store

import (
	"database/sql"
	"fmt"
	"time"

	"task284-boreprobe/internal/model"
)

// CreateProject 新建修复项目并返回带 ID 的实体。
func (d *DB) CreateProject(name, instrumentType string, nominalMM float64) (*model.RestoreProject, error) {
	if name == "" || instrumentType == "" || nominalMM <= 0 {
		return nil, model.NewDomainError("INVALID_INPUT", "项目名、乐器类型与标称内径均必填且大于 0", nil)
	}
	p := &model.RestoreProject{
		Name:           name,
		InstrumentType: instrumentType,
		NominalBoreMM:  nominalMM,
		Status:         model.ProjectPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	res, err := d.Raw.Exec(
		`INSERT INTO projects(name, instrument_type, nominal_bore_mm, status, created_at, updated_at)
		 VALUES(?,?,?,?,?,?)`,
		p.Name, p.InstrumentType, p.NominalBoreMM, p.Status,
		p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "创建项目失败", err)
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

// GetProject 按 ID 读取项目，不存在返回 ErrNotFound。
func (d *DB) GetProject(id int64) (*model.RestoreProject, error) {
	row := d.Raw.QueryRow(
		`SELECT id, name, instrument_type, nominal_bore_mm, status, created_at, updated_at, sealed_at
		 FROM projects WHERE id=?`, id)
	return scanProject(row)
}

func scanProject(row *sql.Row) (*model.RestoreProject, error) {
	var p model.RestoreProject
	var createdAt, updatedAt string
	var sealedAt sql.NullString
	err := row.Scan(&p.ID, &p.Name, &p.InstrumentType, &p.NominalBoreMM, &p.Status, &createdAt, &updatedAt, &sealedAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取项目失败", err)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		p.CreatedAt = t
	}
	if t, e := time.Parse(time.RFC3339Nano, updatedAt); e == nil {
		p.UpdatedAt = t
	}
	if sealedAt.Valid {
		if t, e := time.Parse(time.RFC3339Nano, sealedAt.String); e == nil {
			p.SealedAt = &t
		}
	}
	return &p, nil
}

// ListProjects 按创建时间倒序列出项目。
func (d *DB) ListProjects() ([]*model.RestoreProject, error) {
	rows, err := d.Raw.Query(
		`SELECT id, name, instrument_type, nominal_bore_mm, status, created_at, updated_at, sealed_at
		 FROM projects ORDER BY id DESC`)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出项目失败", err)
	}
	defer rows.Close()
	var out []*model.RestoreProject
	for rows.Next() {
		var p model.RestoreProject
		var createdAt, updatedAt string
		var sealedAt sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.InstrumentType, &p.NominalBoreMM, &p.Status, &createdAt, &updatedAt, &sealedAt); err != nil {
			return nil, err
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			p.CreatedAt = t
		}
		if t, e := time.Parse(time.RFC3339Nano, updatedAt); e == nil {
			p.UpdatedAt = t
		}
		if sealedAt.Valid {
			if t, e := time.Parse(time.RFC3339Nano, sealedAt.String); e == nil {
				p.SealedAt = &t
			}
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// UpdateProjectStatus 更新项目状态；封存状态拒绝再次流转。
func (d *DB) UpdateProjectStatus(id int64, status string) (*model.RestoreProject, error) {
	p, err := d.GetProject(id)
	if err != nil {
		return nil, err
	}
	if p.Status == model.ProjectSealed {
		return nil, model.ErrSealed
	}
	if !model.ValidProjectStatus(status) {
		return nil, model.NewDomainError("INVALID_STATUS", "非法项目状态: "+status, nil)
	}
	var sealedAt any
	if status == model.ProjectSealed {
		sealedAt = time.Now().Format(time.RFC3339Nano)
	}
	if _, err := d.Raw.Exec(
		`UPDATE projects SET status=?, updated_at=?, sealed_at=? WHERE id=?`,
		status, time.Now().Format(time.RFC3339Nano), sealedAt, id); err != nil {
		return nil, model.Wrap("DB_ERROR", "更新项目状态失败", err)
	}
	return d.GetProject(id)
}

// EnsureMutable 校验项目未封存，否则返回 ErrSealed。
func (d *DB) EnsureMutable(id int64) error {
	p, err := d.GetProject(id)
	if err != nil {
		return err
	}
	if p.Status == model.ProjectSealed {
		return fmt.Errorf("ensure mutable: %v", model.ErrSealed)
	}
	return nil
}
