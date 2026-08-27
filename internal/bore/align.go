// Package bore 实现内腔段对齐、尺度校正与连续轮廓构造。
// 核心职责：把若干段扫描在统一轴向坐标上拼成连续内腔轮廓，
// 并检测段间重叠矛盾与尺度单位问题。
package bore

import (
	"fmt"
	"sort"

	"task284-boreprobe/internal/model"
)

// ScaleSpec 描述一个轴向长度单位到毫米的换算。
// 支持 mm / cm；其他单位一律判为尺度不明（ErrScaleUnknown）。
type ScaleSpec struct {
	Unit string
	ToMM float64
}

// ScaleTable 是各单位的换算表。
var ScaleTable = map[string]float64{
	"mm": 1.0,
	"cm": 10.0,
	"m":  1000.0,
}

// ResolveScale 返回单位的毫米换算系数，未知单位返回错误。
func ResolveScale(unit string) (float64, error) {
	if f, ok := ScaleTable[unit]; ok {
		return f, nil
	}
	return 0, model.ErrScaleUnknown
}

// ScaleSegment 把段轴向坐标统一换算到毫米（内径本身已是毫米）。
func ScaleSegment(seg *model.BoreSegment) (startMM, endMM, pitchMM float64, err error) {
	f, err := ResolveScale(seg.Unit)
	if err != nil {
		return 0, 0, 0, model.Wrap("SCALE_UNKNOWN",
			fmt.Sprintf("段 %q 尺度单位 %q 不明", seg.Label, seg.Unit), err)
	}
	return seg.AxialStart * f, seg.AxialEnd * f, seg.PitchMM * f, nil
}

// AlignResult 是段对齐的输出。
type AlignResult struct {
	Bore      *model.AlignedBore
	Overlaps  []OverlapIssue
	Excluded  []string
	ScaleNote string
}

// OverlapIssue 描述段间重叠且内径矛盾的异常。
type OverlapIssue struct {
	LeftLabel  string
	RightLabel string
	StartMM    float64
	EndMM      float64
	LeftAvg    float64
	RightAvg   float64
}

// segWork 是对齐内部的工作单元：原始段 + 换算后的轴向坐标。
type segWork struct {
	seg     *model.BoreSegment
	startMM float64
	endMM   float64
	pitchMM float64
	scaled  []float64
}

// Align 对项目全部段执行对齐：
// 1) 单位统一到 mm；2) 排除 excluded/缺失段；3) 排序；
// 4) 检测轴向重叠；重叠区平均内径偏差超 5% 记为矛盾；
// 5) 把全部有效段采样点拼接为连续轮廓。
func Align(segs []*model.BoreSegment) (*AlignResult, error) {
	res := &AlignResult{Bore: &model.AlignedBore{}}
	var ws []segWork
	for _, s := range segs {
		if s.Status == model.SegmentExcluded {
			res.Excluded = append(res.Excluded, s.Label)
			continue
		}
		st, en, p, err := ScaleSegment(s)
		if err != nil {
			return nil, err
		}
		// 将内径序列按新间距重采样（长度不变，位置由 start 定）。
		scaled := make([]float64, len(s.DiameterMM))
		copy(scaled, s.DiameterMM)
		ws = append(ws, segWork{seg: s, startMM: st, endMM: en, pitchMM: p, scaled: scaled})
		res.Bore.SegmentIDs = append(res.Bore.SegmentIDs, s.ID)
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].startMM < ws[j].startMM })

	// 重叠矛盾检测：两段区间相交时比较重叠区平均内径。
	for i := 0; i < len(ws); i++ {
		for j := i + 1; j < len(ws); j++ {
			lo := max(ws[i].startMM, ws[j].startMM)
			hi := min(ws[i].endMM, ws[j].endMM)
			if hi <= lo {
				continue
			}
			leftAvg, okL := avgInRange(ws[i], lo, hi)
			rightAvg, okR := avgInRange(ws[j], lo, hi)
			if !okL || !okR {
				continue
			}
			diff := (leftAvg - rightAvg) / rightAvg
			if diff > 0.05 || diff < -0.05 {
				res.Overlaps = append(res.Overlaps, OverlapIssue{
					LeftLabel: ws[i].seg.Label, RightLabel: ws[j].seg.Label,
					StartMM: lo, EndMM: hi, LeftAvg: leftAvg, RightAvg: rightAvg,
				})
			}
		}
	}
	if len(res.Overlaps) > 0 {
		return nil, model.NewDomainError("OVERLAP_CONTRADICT",
			fmt.Sprintf("检测到 %d 处段重叠内径矛盾", len(res.Overlaps)), model.ErrOverlapContradict)
	}

	// 拼接连续轮廓。
	for _, w := range ws {
		n := len(w.scaled)
		for k := 0; k < n; k++ {
			res.Bore.Points = append(res.Bore.Points, model.BorePoint{
				AxialMM:     w.startMM + float64(k)*w.pitchMM,
				DiameterMM:  w.scaled[k],
				SegmentID:   w.seg.ID,
				SegmentLabel: w.seg.Label,
			})
		}
	}
	res.Bore.ProjectID = projectIDOf(segs)
	res.Bore.ScaleNote = "全部段已统一换算到 mm 并排序拼接"
	return res, nil
}

func projectIDOf(segs []*model.BoreSegment) int64 {
	if len(segs) == 0 {
		return 0
	}
	return segs[0].ProjectID
}

// avgInRange 计算某段在 [lo,hi] 区间内的平均内径。
func avgInRange(w segWork, lo, hi float64) (float64, bool) {
	var sum float64
	var n int
	for k, v := range w.scaled {
		x := w.startMM + float64(k)*w.pitchMM
		if x >= lo && x <= hi {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}
