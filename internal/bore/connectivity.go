package bore

import (
	"fmt"
	"math"
	"sort"

	"task284-boreprobe/internal/model"
)

// CheckConnectivity 基于对齐轮廓检查管腔连通性：
// 1) 相邻采样点内径突变超过 25% 记为断点（diameter_drop）；
// 2) 相邻采样点轴向间距超过参考间距 3 倍记为数据缺口（missing_scan），
//    参考间距取全部相邻间距的中位数（对异常值鲁棒）；
// 3) 返回最小内径、缩窄比与断点清单。
// 供裁决使用：缩窄比 < threshold 时判定为「缩窄」。
func CheckConnectivity(bore *model.AlignedBore, nominalMM float64) (*model.ConnectivityReport, error) {
	pts := bore.Points
	if len(pts) == 0 {
		return nil, model.NewDomainError("NO_BORE", "尚无对齐后的内腔轮廓", nil)
	}
	report := &model.ConnectivityReport{
		ProjectID:       bore.ProjectID,
		NominalDiameter: nominalMM,
		MinDiameterMM:   math.MaxFloat64,
	}
	refPitch := medianSpacing(pts)
	var prev *model.BorePoint
	for i := range pts {
		p := &pts[i]
		if p.DiameterMM < report.MinDiameterMM {
			report.MinDiameterMM = p.DiameterMM
		}
		if prev != nil {
			drop := (prev.DiameterMM - p.DiameterMM) / prev.DiameterMM
			if drop > 0.25 {
				report.Gaps = append(report.Gaps, model.Gap{
					AxialStart: prev.AxialMM, AxialEnd: p.AxialMM,
					Reason: fmt.Sprintf("diameter_drop(%.1f%% -> %.1f%%)", prev.DiameterMM, p.DiameterMM),
				})
			}
			if refPitch > 0 && p.AxialMM-prev.AxialMM > 3*refPitch {
				report.Gaps = append(report.Gaps, model.Gap{
					AxialStart: prev.AxialMM, AxialEnd: p.AxialMM,
					Reason: "missing_scan(轴向间距异常)",
				})
			}
		}
		prev = p
	}
	if report.MinDiameterMM == math.MaxFloat64 {
		report.MinDiameterMM = 0
	}
	if nominalMM > 0 {
		report.NarrowRatio = report.MinDiameterMM / nominalMM
	}
	return report, nil
}

// medianSpacing 返回相邻采样点轴向间距的中位数；不足 2 点时返回 0。
func medianSpacing(pts []model.BorePoint) float64 {
	if len(pts) < 2 {
		return 0
	}
	spacings := make([]float64, 0, len(pts)-1)
	for i := 1; i < len(pts); i++ {
		spacings = append(spacings, pts[i].AxialMM-pts[i-1].AxialMM)
	}
	sort.Float64s(spacings)
	n := len(spacings)
	if n%2 == 1 {
		return spacings[n/2]
	}
	return (spacings[n/2-1] + spacings[n/2]) / 2
}

// PatchInRange 校验补片区间是否落在已对齐轮廓的覆盖范围内。
// 越界返回 ErrPatchOutOfRange，用于补片导入与裁决前置检查。
func PatchInRange(bore *model.AlignedBore, p *model.Patch) error {
	if len(bore.Points) == 0 {
		return model.ErrPatchOutOfRange
	}
	lo := bore.Points[0].AxialMM
	hi := bore.Points[len(bore.Points)-1].AxialMM
	if p.AxialStart < lo-0.01 || p.AxialEnd > hi+0.01 {
		return model.NewDomainError("PATCH_OUT_OF_RANGE",
			fmt.Sprintf("补片区间 [%.1f,%.1f] 超出内腔覆盖 [%.1f,%.1f]", p.AxialStart, p.AxialEnd, lo, hi),
			model.ErrPatchOutOfRange)
	}
	return nil
}
