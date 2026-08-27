// Package store 提供 SQLite 持久化：建表迁移、事务与各实体 CRUD。
// 使用纯 Go 驱动 modernc.org/sqlite，CGO 无关，离线可构建。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DB 封装 *sql.DB 并提供迁移与事务助手。
type DB struct {
	Raw         *sql.DB
	segmentMu   sync.Mutex
	responseMu  sync.Mutex
}

// Open 打开（必要时创建）SQLite 数据库并执行迁移。
// 若 dbPath 为 ":memory:" 则使用内存库（测试用）。
func Open(dbPath string) (*DB, error) {
	if dbPath != ":memory:" {
		if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("创建数据库目录: %w", err)
			}
		}
	}
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	}
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	raw.SetMaxOpenConns(1) // SQLite 单写者，串行化连接避免锁竞争
	d := &DB{Raw: raw}
	if err := d.migrate(); err != nil {
		raw.Close()
		return nil, err
	}
	return d, nil
}

// Close 关闭数据库连接。
func (d *DB) Close() error { return d.Raw.Close() }

// migrate 以幂等方式创建全部表结构（IF NOT EXISTS）。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			instrument_type TEXT NOT NULL,
			nominal_bore_mm REAL NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			sealed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS segments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			label TEXT NOT NULL,
			axial_start REAL NOT NULL,
			axial_end REAL NOT NULL,
			diameter_mm TEXT NOT NULL,
			pitch_mm REAL NOT NULL,
			unit TEXT NOT NULL DEFAULT 'mm',
			status TEXT NOT NULL DEFAULT 'raw',
			hash TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_segments_project ON segments(project_id)`,
		`CREATE TABLE IF NOT EXISTS patches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			axial_start REAL NOT NULL,
			axial_end REAL NOT NULL,
			thickness_mm REAL NOT NULL,
			material TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_patches_project ON patches(project_id)`,
		`CREATE TABLE IF NOT EXISTS responses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			kind TEXT NOT NULL,
			base_freq_hz REAL NOT NULL,
			peaks TEXT NOT NULL,
			capture_at TEXT NOT NULL DEFAULT '',
			hash TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_responses_project ON responses(project_id)`,
		`CREATE TABLE IF NOT EXISTS connectivity_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			min_diameter_mm REAL NOT NULL,
			nominal_diameter_mm REAL NOT NULL,
			narrow_ratio REAL NOT NULL,
			gaps TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_connectivity_project ON connectivity_reports(project_id)`,
		`CREATE TABLE IF NOT EXISTS impacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			patch_id INTEGER NOT NULL REFERENCES patches(id),
			kind TEXT NOT NULL DEFAULT 'candidate',
			narrow_ratio REAL NOT NULL,
			pitch_shift_cents REAL NOT NULL,
			evidence TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_impacts_project ON impacts(project_id)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			base_hash TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at TEXT NOT NULL,
			frozen_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_project ON snapshots(project_id)`,
		`CREATE TABLE IF NOT EXISTS compare_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			before_id INTEGER NOT NULL,
			after_id INTEGER NOT NULL,
			pitch_shift_cents REAL NOT NULL,
			residual_rms REAL NOT NULL,
			significant INTEGER NOT NULL,
			detail TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_compare_project ON compare_results(project_id)`,
	}
	for _, s := range stmts {
		if _, err := d.Raw.Exec(s); err != nil {
			return fmt.Errorf("迁移失败: %w", err)
		}
	}
	return nil
}

// Tx 在单个事务中执行 fn；任一步出错则回滚。
func (d *DB) Tx(fn func(tx *sql.Tx) error) error {
	tx, err := d.Raw.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("commit after fn error: %v (fn=%w)", commitErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
