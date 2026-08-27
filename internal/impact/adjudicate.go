// Package impact 实现补片修复影响的裁决：
// 综合连通性缩窄、频响偏移与补片越界检查，给出可解释结论。
package impact

import (
	"fmt"

	"task284-boreprobe/internal/model"
)

// Adjudication 是一次裁决的输入与输出。
type Adjudication struct {
	Patch           *model.Patch
	Connectivity    *model.ConnectivityReport
	Compare         *model.CompareResult
	NarrowThreshold float64 // 缩窄判定阈值（内径占比，默认 0.9）
}

// Decision 是裁决结果。
type Decision struct {
	Kind            string  `json:"kind"` // none / narrowed / conflict
	NarrowRatio     float64 `json:"narrow_ratio"`
	PitchShiftCents float64 `json:"pitch_shift_cents"`
	Evidence        string  `json:"evidence"`
}

// NarrowThreshold 默认缩窄判定阈值。
const NarrowThreshold = 0.9

// Adjudicate 裁决补片影响：
// 1) 补片覆盖区内径占比 < 阈值 → narrowed；
// 2) 否则若频响比较显著 → conflict；
// 3) 否则 → none。
// 证据文本必须能解释「为什么是这个结论」。
func Adjudicate(a *Adjudication) (*Decision, error) {
	th := a.NarrowThreshold
	if th <= 0 {
		th = NarrowThreshold
	}
	if a.Patch == nil {
		return nil, model.NewDomainError("PATCH_REQUIRED", "裁决缺少补片", nil)
	}
	var ratio float64 = 1.0
	var connEvidence string
	if a.Connectivity != nil {
		ratio = a.Connectivity.NarrowRatio
		connEvidence = fmt.Sprintf("全管腔最小内径占比 %.1f%%", ratio*100)
	}
	var shift float64
	var cmpEvidence string
	if a.Compare != nil {
		shift = a.Compare.PitchShiftCents
		cmpEvidence = fmt.Sprintf("修复前后基频偏移 %.1f 音分（显著=%v）", shift, a.Compare.Significant)
	}
	switch {
	case ratio < th:
		evidence := fmt.Sprintf("补片 %s 覆盖区间内径缩窄至标称 %.1f%%（%s），判定为缩窄",
			a.Patch.Material, ratio*100, connEvidence)
		if a.Compare != nil && a.Compare.Significant {
			evidence += "；同时存在" + cmpEvidence
		}
		return &Decision{Kind: model.ImpactNarrowed, NarrowRatio: ratio,
			PitchShiftCents: shift, Evidence: evidence}, nil
	case a.Compare != nil && a.Compare.Significant:
		evidence := fmt.Sprintf("补片区域未见缩窄（%s），但%s，判定为响应冲突",
			connEvidence, cmpEvidence)
		return &Decision{Kind: model.ImpactConflict, NarrowRatio: ratio,
			PitchShiftCents: shift, Evidence: evidence}, nil
	default:
		evidence := fmt.Sprintf("补片区域未缩窄（%s）且%s，判定为无影响",
			connEvidence, cmpEvidence)
		return &Decision{Kind: model.ImpactNone, NarrowRatio: ratio,
			PitchShiftCents: shift, Evidence: evidence}, nil
	}
}
