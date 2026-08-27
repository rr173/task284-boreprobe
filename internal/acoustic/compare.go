package acoustic

import (
	"fmt"
	"sort"

	"task284-boreprobe/internal/model"
)

// ResolvePair 从项目频响观测中选出 before/after 各一条用于比较。
// 若存在多组，取最新的 before 与最新的 after。
func ResolvePair(resps []*model.FrequencyResponse) (*model.FrequencyResponse, *model.FrequencyResponse, error) {
	var before, after *model.FrequencyResponse
	for _, r := range resps {
		switch r.Kind {
		case "before":
			if before == nil || r.ID > before.ID {
				before = r
			}
		case "after":
			if after == nil || r.ID > after.ID {
				after = r
			}
		}
	}
	if before == nil || after == nil {
		return nil, nil, model.NewDomainError("PAIR_INCOMPLETE",
			"需要同时存在 before 与 after 频响观测才能比较", nil)
	}
	return before, after, nil
}

// RankShifts 按 |音分偏移| 降序排列峰匹配结果，便于定位偏移最大的谐振峰。
func RankShifts(shifts []PeakShift) []PeakShift {
	out := make([]PeakShift, len(shifts))
	copy(out, shifts)
	sort.SliceStable(out, func(i, j int) bool {
		return absF(out[i].DevCents) > absF(out[j].DevCents)
	})
	return out
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// SummarizeCompare 生成人类可读的比较结论（用于裁决证据）。
func SummarizeCompare(res *model.CompareResult) string {
	verdict := "无显著差异"
	if res.Significant {
		dir := "升高"
		if res.PitchShiftCents < 0 {
			dir = "降低"
		}
		verdict = fmt.Sprintf("音高显著%s %.1f 音分", dir, absF(res.PitchShiftCents))
	}
	return fmt.Sprintf("%s；峰残差 RMS %.2f 音分", verdict, res.ResidualRMS)
}
