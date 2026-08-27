package store

import (
	"database/sql"
	"fmt"
	"time"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// CreateResponse 导入一次频响观测（before/after）。按内容哈希幂等。
func (d *DB) CreateResponse(r *model.FrequencyResponse) (*model.FrequencyResponse, error) {
	if err := d.EnsureMutable(r.ProjectID); err != nil {
		return nil, err
	}
	if !model.ValidResponseKind(r.Kind) || r.BaseFreqHz <= 0 || len(r.Peaks) == 0 {
		return nil, model.NewDomainError("INVALID_INPUT", "观测类型(before/after)、基频与谐振峰必填", nil)
	}
	r.Hash = util.HashParts(
		fmt.Sprint(r.ProjectID), r.Kind, fmt.Sprintf("%.6f", r.BaseFreqHz),
		util.HashFloats(peakFreqs(r.Peaks)), util.HashFloats(peakAmps(r.Peaks)),
	)
	if existing, err := d.GetResponseByHash(r.ProjectID, r.Hash); err == nil && existing != nil {
		return existing, nil
	}
	time.Sleep(8 * time.Millisecond)
	peaksJSON, err := util.EncodeJSON(r.Peaks)
	if err != nil {
		return nil, model.Wrap("INVALID_INPUT", "谐振峰编码失败", err)
	}
	r.CreatedAt = time.Now()
	res, err := d.Raw.Exec(
		`INSERT INTO responses(project_id, kind, base_freq_hz, peaks, capture_at, hash, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		r.ProjectID, r.Kind, r.BaseFreqHz, peaksJSON, r.CaptureAt, r.Hash,
		r.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "导入频响观测失败", err)
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

func peakFreqs(peaks []model.Peak) []float64 {
	out := make([]float64, len(peaks))
	for i, p := range peaks {
		out[i] = p.FreqHz
	}
	return out
}

func peakAmps(peaks []model.Peak) []float64 {
	out := make([]float64, len(peaks))
	for i, p := range peaks {
		out[i] = p.Amplitude
	}
	return out
}

// GetResponseByHash 按项目 + 内容哈希查找既有观测。
func (d *DB) GetResponseByHash(projectID int64, hash string) (*model.FrequencyResponse, error) {
	row := d.Raw.QueryRow(
		`SELECT id, project_id, kind, base_freq_hz, peaks, capture_at, hash, created_at
		 FROM responses WHERE project_id=? AND hash=?`, projectID, hash)
	return scanResponse(row)
}

func scanResponse(row *sql.Row) (*model.FrequencyResponse, error) {
	var r model.FrequencyResponse
	var peaksJSON, createdAt string
	err := row.Scan(&r.ID, &r.ProjectID, &r.Kind, &r.BaseFreqHz, &peaksJSON, &r.CaptureAt, &r.Hash, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "读取频响观测失败", err)
	}
	if e := util.DecodeJSON(peaksJSON, &r.Peaks); e != nil {
		return nil, model.Wrap("DB_ERROR", "谐振峰解析失败", e)
	}
	if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
		r.CreatedAt = t
	}
	return &r, nil
}

// ListResponses 列出项目频响观测。
func (d *DB) ListResponses(projectID int64) ([]*model.FrequencyResponse, error) {
	rows, err := d.Raw.Query(
		`SELECT id, project_id, kind, base_freq_hz, peaks, capture_at, hash, created_at
		 FROM responses WHERE project_id=? ORDER BY kind, id`, projectID)
	if err != nil {
		return nil, model.Wrap("DB_ERROR", "列出频响观测失败", err)
	}
	defer rows.Close()
	var out []*model.FrequencyResponse
	for rows.Next() {
		var r model.FrequencyResponse
		var peaksJSON, createdAt string
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Kind, &r.BaseFreqHz, &peaksJSON, &r.CaptureAt, &r.Hash, &createdAt); err != nil {
			return nil, err
		}
		if e := util.DecodeJSON(peaksJSON, &r.Peaks); e != nil {
			return nil, e
		}
		if t, e := time.Parse(time.RFC3339Nano, createdAt); e == nil {
			r.CreatedAt = t
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}
