package store

import (
	"database/sql"
	"errors"
	"testing"

	"task284-boreprobe/internal/model"
)

func TestTxRollsBackOnFailure(t *testing.T) {
	db := setupDB(t)
	p, _ := db.CreateProject("事务回滚", "oboe", 14.5)
	snap, err := db.CreateSnapshot(&model.RestoreSnapshot{
		ProjectID: p.ID, Name: "frozen-v1", Payload: `{}`, BaseHash: "abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.TransitionSnapshot(snap.ID, model.SnapshotFrozen); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("supersede failed")
	err = db.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO snapshots(project_id, name, status, base_hash, payload, created_at)
			 VALUES(?,?,?,?,?,?)`,
			p.ID, "draft-v2", model.SnapshotDraft, "def", `{}`, snap.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		)
		if err != nil {
			return err
		}
		newID, _ := res.LastInsertId()
		if _, err := tx.Exec(
			`UPDATE snapshots SET status='superseded' WHERE project_id=? AND id<>? AND status IN ('frozen','shared')`,
			p.ID, newID,
		); err != nil {
			return err
		}
		return boom
	})
	if err == nil {
		t.Fatal("期望事务失败")
	}
	var n int
	if err := db.Raw.QueryRow(`SELECT COUNT(*) FROM snapshots WHERE project_id=? AND name='draft-v2'`, p.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("失败事务不应留下草稿快照，实际 %d", n)
	}
	row := db.Raw.QueryRow(`SELECT status FROM snapshots WHERE id=?`, snap.ID)
	var status string
	if err := row.Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != model.SnapshotFrozen {
		t.Fatalf("旧冻结快照应保持 frozen，实际 %s", status)
	}
}
