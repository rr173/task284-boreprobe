package bore

import (
	"math"

	"task284-boreprobe/internal/model"
)

// ProfileStats 是连续内腔轮廓的统计摘要。
type ProfileStats struct {
	TotalLengthMM   float64     `json:"total_length_mm"`
	MinDiameterMM   float64     `json:"min_diameter_mm"`
	MaxDiameterMM   float64     `json:"max_diameter_mm"`
	MeanDiameterMM  float64     `json:"mean_diameter_mm"`
	NarrowZones     []NarrowZone `json:"narrow_zones"`
	SegmentCoverage float64     `json:"segment_coverage"` // 有效段覆盖的轴向占比
}

// NarrowZone 是相对标称内径缩窄到阈值以下的连续区域。
type NarrowZone struct {
	StartMM    float64 `json:"start_mm"`
	EndMM      float64 `json:"end_mm"`
	MinRatio   float64 `json:"min_ratio"`
	PatchOverlap bool   `json:"patch_overlap"` // 与补片区间重叠
}

// AnalyzeProfile 从对齐轮廓计算统计与缩窄区。
// threshold 为缩窄判定的内径占比阈值（如 0.9 = 缩到标称 90% 以下）。
func AnalyzeProfile(bore *model.AlignedBore, nominalMM, threshold float64) *ProfileStats {
	pts := bore.Points
	if len(pts) == 0 {
		return &ProfileStats{}
	}
	stats := &ProfileStats{
		MinDiameterMM:  math.MaxFloat64,
		MaxDiameterMM:  0,
		SegmentCoverage: 0,
	}
	var sum float64
	var count int
	var zoneStart *float64
	zoneMin := math.MaxFloat64
	flush := func() {
		if zoneStart != nil {
			stats.NarrowZones = append(stats.NarrowZones, NarrowZone{
				StartMM: *zoneStart, EndMM: pts[count-1].AxialMM, MinRatio: zoneMin / nominalMM,
			})
			zoneStart = nil
			zoneMin = math.MaxFloat64
		}
	}
	for i, p := range pts {
		if p.DiameterMM < stats.MinDiameterMM {
			stats.MinDiameterMM = p.DiameterMM
		}
		if p.DiameterMM > stats.MaxDiameterMM {
			stats.MaxDiameterMM = p.DiameterMM
		}
		sum += p.DiameterMM
		count++
		if p.DiameterMM < nominalMM*threshold {
			if zoneStart == nil {
				z := p.AxialMM
				zoneStart = &z
			}
			if p.DiameterMM < zoneMin {
				zoneMin = p.DiameterMM
			}
		} else {
			flush()
		}
		_ = i
	}
	flush()
	if count > 0 {
		stats.MeanDiameterMM = sum / float64(count)
		if len(pts) >= 2 {
			stats.TotalLengthMM = pts[len(pts)-1].AxialMM - pts[0].AxialMM
			stats.SegmentCoverage = 1.0
		}
	} else {
		stats.MinDiameterMM = 0
	}
	return stats
}

// MarkPatchOverlap 标记缩窄区与补片的重叠关系。
func MarkPatchOverlap(zones []NarrowZone, patches []*model.Patch) {
	for i := range zones {
		z := &zones[i]
		for _, p := range patches {
			if z.StartMM <= p.AxialEnd && z.EndMM >= p.AxialStart {
				z.PatchOverlap = true
				break
			}
		}
	}
}
