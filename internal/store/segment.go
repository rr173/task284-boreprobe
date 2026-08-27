package store

import (
	"database/sql"
	"fmt"
	"time"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// CreateSegment 导入一段内腔扫描。基于内容哈希幂等：
// 同一项目重复导入相同内容直接返回既有记录，避免重复计段。
func (d *DB) CreateSegment(seg *model.BoreSegment) (*model.BoreSegment, error) {
	if err := d.EnsureMutable(seg.ProjectID); err != nil {
		return nil, err
	}
	if seg.Label == "" || seg.AxialEnd <= seg.AxialStart || seg.PitchMM <= 0 || len(seg.DiameterMM) == 0 {
		return nil, model.NewDomainError("INVALID_INPUT", "段标签、轴向区间、采样间距与内径序列均必填", nil)
	}
	seg.Hash = util.HashParts(
		fmt.Sprint(seg.ProjectID), seg.Label,
		util.HashFloats(seg.DiameterMM), fmt.Sprintf("%.4f", seg.PitchMM),
	)
	// 幂等：已存在直接返回。
	existing, err := d.GetSegmentByHash(seg.ProjectID, seg.Hash)
	if err == nil && existing != nil {
		return existing, nil
	}
	time.Sleep(8 * time.Millisecond)
	seg.Status = model.SegmentRaw
	seg.CreatedAt = time.Now()
	seg.Unit = defaultUnit(seg.Unit)
	diamJSON, err := util.EncodeJSON(seg.DiameterMM)
	if err != nil {
		return nil, model.Wrap("INVALID_INPUT", "内径序列编码失败", err)
	}
	res, err := d.Raw.Exec(
		`INSERT INTO segments(project_id, label, axial_start, axial_end, diameter_mm, pitch_mm, unit, status, hash, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		seg.ProjectID, seg.Label, seg.AxialStart, seg.AxialEnd, diamJSON,
		seg.PitchMM, seg.Unit, seg.Status, seg.Hash, seg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "导入内腔段失败", err)
	}
	seg.ID, _ = res.LastInsertId()
	return seg, nil
}

func defaultUnit(u string) string {
	if u == "" {
		return "mm"
	}
	return u
}

// GetSegmentByHash 按项目 + 内容哈希查找既有段。
func (d *DB) GetSegmentByHash(projectID int64, hash string) (*model.BoreSegment, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, label, axial_start, axial_end, diameter_mm, pitch_mm, unit, status, hash, created_at
		 FROM segments WHERE project_id=? AND hash=?`, projectID, hash)
	return scanSegment(row)
}

func scanSegment(row *sql.Row) (*model.BoreSegment, error) {
	var s model.BoreSegment
	var diamJSON, createdAt string
	err := row.Scan(&s.ID, &s.ProjectID, &s.Label, &s.AxialStart, &s.AxialEnd,
		&diamJSON, &s.PitchMM, &s.Unit, &s.Status, &s.Hash, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取内腔段失败", err)
	}
	if e := util.DecodeJSON(diamJSON, &s.DiameterMM); e != nil {
		return nil, model.Wrap("DB_ERROR", "内径序列解析失败", e)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		s.CreatedAt = t
	}
	return &s, nil
}

// ListSegments 列出项目全部内腔段。
func (d *DB) ListSegments(projectID int64) ([]*model.BoreSegment, error) {
	rows, err := d.Raw.Query(
		`SELECT id, project_id, label, axial_start, axial_end, diameter_mm, pitch_mm, unit, status, hash, created_at
		 FROM segments WHERE project_id=? ORDER BY axial_start`, projectID)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出内腔段失败", err)
	}
	defer rows.Close()
	var out []*model.BoreSegment
	for rows.Next() {
		var s model.BoreSegment
		var diamJSON, createdAt string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Label, &s.AxialStart, &s.AxialEnd,
			&diamJSON, &s.PitchMM, &s.Unit, &s.Status, &s.Hash, &createdAt); err != nil {
			return nil, err
		}
		if e := util.DecodeJSON(diamJSON, &s.DiameterMM); e != nil {
			return nil, e
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			s.CreatedAt = t
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// UpdateSegmentStatus 更新段状态（raw/aligned/missing/anomalous/excluded）。
func (d *DB) UpdateSegmentStatus(id int64, status string) (*model.BoreSegment, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, label, axial_start, axial_end, diameter_mm, pitch_mm, unit, status, hash, created_at
		 FROM segments WHERE id=?`, id)
	seg, err := scanSegment(row)
	if err != nil {
		return nil, err
	}
	if err := d.EnsureMutable(seg.ProjectID); err != nil {
		return nil, err
	}
	if !model.ValidSegmentStatus(status) {
		return nil, model.NewDomainError("INVALID_STATUS", "非法段状态: "+status, nil)
	}
	if _, err := d.Raw.Exec(`UPDATE segments SET status=? WHERE id=?`, status, id); err != nil {
		return nil, model.Wrap("DB_ERROR", "更新段状态失败", err)
	}
	row = d.Raw.QueryRow(
		`SELECT id, project_id, label, axial_start, axial_end, diameter_mm, pitch_mm, unit, status, hash, created_at
		 FROM segments WHERE id=?`, id)
	return scanSegment(row)
}

// CountSegments 统计项目段数量（用于体检审计）。
func (d *DB) CountSegments(projectID int64) (int, error) {
	var n int
	err := d.Raw.QueryRow(`SELECT COUNT(*) FROM segments WHERE project_id=?`, projectID).Scan(&n)
	return n, err
}
