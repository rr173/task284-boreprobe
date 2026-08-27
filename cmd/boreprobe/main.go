// Command boreprobe 是历史木管乐器内腔修复证据复核台的服务入口。
// 支持 --addr / --db / --smoke-test 三个标志：
//   - --smoke-test 执行端到端自检后退出（不启动长驻服务）。
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/httpapi"
	"task284-boreprobe/internal/service"
	"task284-boreprobe/internal/store"
)

func main() {
	var addr, dbPath string
	var smoke bool
	flag.StringVar(&addr, "addr", ":8080", "HTTP 监听地址")
	flag.StringVar(&dbPath, "db", "boreprobe.db", "SQLite 数据库路径")
	flag.BoolVar(&smoke, "smoke-test", false, "执行端到端自检后退出，不启动长驻服务")
	flag.Parse()

	if smoke {
		if err := runSmoke(dbPath); err != nil {
			fmt.Fprintln(os.Stderr, "smoke-test FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("smoke-test PASSED")
		os.Exit(0)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}
	defer db.Close()

	svc := service.New(db)
	mux := httpapi.New(svc).Handler()
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Println("boreprobe listening on", addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

// runSmoke 端到端自检：真实创建项目、导入内腔段/补片/频响、执行对齐与
// 连通性检查、比较频响、裁决影响、发布冻结快照、封存项目；
// 随后关闭数据库并重新打开，验证数据持久化与重启恢复，最后以 nil 退出。
func runSmoke(dbPath string) error {
	// 清理上次自检残留，保证可重复执行。
	_ = os.Remove(dbPath)
	for _, ext := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + ext)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	svc := service.New(db)

	// 1. 创建修复项目（双簧管，标称内径 14.5mm）。
	proj, err := svc.CreateProject("巴洛克双簧管首修复核", "oboe", 14.5)
	if err != nil {
		return fmt.Errorf("创建项目: %w", err)
	}

	// 2. 导入两段内腔扫描（单位 mm，轴向 0-60 与 60-120）。
	seg1, err := svc.AddSegment(&model.BoreSegment{
		ProjectID: proj.ID, Label: "上节", AxialStart: 0, AxialEnd: 60, PitchMM: 2, Unit: "mm",
		DiameterMM: []float64{14.5, 14.5, 14.4, 14.5, 14.3, 14.4, 14.5, 14.5, 14.4, 14.5,
			14.5, 14.5, 14.4, 14.5, 14.3, 14.4, 14.5, 14.5, 14.4, 14.5,
			14.5, 14.5, 14.4, 14.5, 14.3, 14.4, 14.5, 14.5, 14.4, 14.5},
	})
	if err != nil {
		return fmt.Errorf("导入段1: %w", err)
	}
	seg2, err := svc.AddSegment(&model.BoreSegment{
		ProjectID: proj.ID, Label: "下节", AxialStart: 60, AxialEnd: 120, PitchMM: 2, Unit: "mm",
		// 70-90mm（索引 5..14）为补片覆盖区，内径缩窄到 12.5mm，
		// 使连通性检查能定位到显著缩窄（12.5/14.5 ≈ 86% < 90%）。
		DiameterMM: []float64{14.5, 14.5, 14.4, 14.5, 14.3, 12.5, 12.5, 12.5, 12.5, 12.5,
			12.5, 12.5, 12.5, 12.5, 12.5, 14.4, 14.5, 14.5, 14.4, 14.5,
			14.5, 14.5, 14.4, 14.5, 14.3, 14.4, 14.5, 14.5, 14.4, 14.5},
	})
	if err != nil {
		return fmt.Errorf("导入段2: %w", err)
	}
	_ = seg1
	_ = seg2

	// 3. 导入补片：下节 70-90mm 区间，厚度 1.2mm。
	patch, err := svc.AddPatch(&model.Patch{
		ProjectID: proj.ID, AxialStart: 70, AxialEnd: 90, ThicknessMM: 1.2,
		Material: "欧洲枫木", Note: "内壁裂缝填补",
	})
	if err != nil {
		return fmt.Errorf("导入补片: %w", err)
	}

	// 4. 导入频响观测（修复前基频 440Hz，修复后 442Hz → +7.8 音分，应显著）。
	if _, err := svc.AddResponse(&model.FrequencyResponse{
		ProjectID: proj.ID, Kind: "before", BaseFreqHz: 440.0,
		Peaks: []model.Peak{
			{FreqHz: 440, Amplitude: 1.0}, {FreqHz: 880, Amplitude: 0.6},
			{FreqHz: 1320, Amplitude: 0.3},
		},
		CaptureAt: "2026-08-20T10:00:00+08:00",
	}); err != nil {
		return fmt.Errorf("导入修复前频响: %w", err)
	}
	if _, err := svc.AddResponse(&model.FrequencyResponse{
		ProjectID: proj.ID, Kind: "after", BaseFreqHz: 442.0,
		Peaks: []model.Peak{
			{FreqHz: 442, Amplitude: 1.0}, {FreqHz: 884, Amplitude: 0.6},
			{FreqHz: 1326, Amplitude: 0.3},
		},
		CaptureAt: "2026-08-22T15:00:00+08:00",
	}); err != nil {
		return fmt.Errorf("导入修复后频响: %w", err)
	}

	// 5. 执行对齐 + 连通性检查。
	bore, conn, err := svc.AlignAndCheck(proj.ID)
	if err != nil {
		return fmt.Errorf("对齐与连通性检查: %w", err)
	}
	if len(bore.Points) == 0 {
		return fmt.Errorf("对齐轮廓为空")
	}
	if conn.NarrowRatio > 1.0+1e-9 {
		return fmt.Errorf("缩窄比异常: %v", conn.NarrowRatio)
	}

	// 6. 频响比较。
	cmp, err := svc.Compare(proj.ID)
	if err != nil {
		return fmt.Errorf("频响比较: %w", err)
	}
	if !cmp.Significant {
		return fmt.Errorf("期望频响差异显著，实际不显著")
	}

	// 7. 创建影响候选并裁决。CreateImpact 已按缩窄优先给出候选证据
	//    （12.5/14.5 ≈ 86% < 90% → narrowed），研究者据此裁决为缩窄。
	imp, err := svc.CreateImpact(proj.ID, patch.ID)
	if err != nil {
		return fmt.Errorf("创建影响候选: %w", err)
	}
	adjudicated, err := svc.AdjudicateImpact(imp.ID, model.ImpactNarrowed, imp.Version)
	if err != nil {
		return fmt.Errorf("裁决影响: %w", err)
	}
	if _, err := svc.ConfirmImpact(adjudicated.ID, adjudicated.Version); err != nil {
		return fmt.Errorf("确认影响: %w", err)
	}

	// 8. 发布冻结快照。
	snap, err := svc.PublishSnapshot(proj.ID, "首修复核 v1")
	if err != nil {
		return fmt.Errorf("发布快照: %w", err)
	}
	if snap.Status != model.SnapshotFrozen {
		return fmt.Errorf("快照应为 frozen，实际 %s", snap.Status)
	}

	// 9. 关闭数据库，重新打开验证持久化与重启恢复。
	if err := db.Close(); err != nil {
		return fmt.Errorf("关闭数据库: %w", err)
	}
	db2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("重开数据库: %w", err)
	}
	defer db2.Close()
	svc2 := service.New(db2)

	reopened, err := svc2.GetProject(proj.ID)
	if err != nil {
		return fmt.Errorf("重开后读取项目: %w", err)
	}
	if reopened.Status != model.ProjectPublished {
		return fmt.Errorf("重开后状态应为 published，实际 %s", reopened.Status)
	}
	segsAfter, err := svc2.ListSegments(proj.ID)
	if err != nil || len(segsAfter) != 2 {
		return fmt.Errorf("重开后段数量应为 2，实际 %d (err=%v)", len(segsAfter), err)
	}
	snapsAfter, err := svc2.ListSnapshots(proj.ID)
	if err != nil || len(snapsAfter) == 0 {
		return fmt.Errorf("重开后快照缺失 (err=%v)", err)
	}

	// 10. 封存项目（基准一致才能封存）。
	sealed, err := svc2.SealProject(proj.ID)
	if err != nil {
		return fmt.Errorf("封存项目: %w", err)
	}
	if sealed.Status != model.ProjectSealed {
		return fmt.Errorf("封存后状态应为 sealed，实际 %s", sealed.Status)
	}
	// 封存后修改必须被拒绝。
	if _, err := svc2.AddPatch(&model.Patch{
		ProjectID: proj.ID, AxialStart: 10, AxialEnd: 20, ThicknessMM: 0.5, Material: "试验",
	}); err == nil {
		return fmt.Errorf("封存后应拒绝新增补片")
	}

	fmt.Printf("自检通过：项目#%d %s；段 %d；补片 %d；影响裁决 %s；快照 %d 份（%s）；封存 %s\n",
		reopened.ID, reopened.Name, len(segsAfter), 1, adjudicated.Kind,
		len(snapsAfter), snap.Name, sealed.Status)
	return nil
}
