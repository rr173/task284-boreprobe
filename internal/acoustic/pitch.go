// Package acoustic 实现修复前后频响的比较：
// 基频偏移（音分）、谐振峰残差与显著性判定。
package acoustic

import (
	"fmt"
	"math"

	"task284-boreprobe/internal/model"
)

// CentsPerOctave 一音分 = 2^(1/1200)。
const CentsPerOctave = 1200.0

// PitchShiftCents 计算两个基频之间的音分偏移。
// 结果为正表示修复后音高上升，负表示下降。
func PitchShiftCents(beforeHz, afterHz float64) float64 {
	if beforeHz <= 0 || afterHz <= 0 {
		return 0
	}
	return CentsPerOctave * math.Log2(afterHz/beforeHz)
}

// MatchPeak 在峰列表中找到与目标频率最近的峰。
func MatchPeak(peaks []model.Peak, targetHz float64) (model.Peak, float64, bool) {
	if len(peaks) == 0 {
		return model.Peak{}, 0, false
	}
	best := peaks[0]
	bestDev := math.Abs(best.FreqHz - targetHz)
	for _, p := range peaks[1:] {
		if d := math.Abs(p.FreqHz - targetHz); d < bestDev {
			best = p
			bestDev = d
		}
	}
	return best, bestDev, true
}

// PeakShift 是单个谐振峰的匹配结果。
type PeakShift struct {
	BeforeHz    float64 `json:"before_hz"`
	AfterHz     float64 `json:"after_hz"`
	DeviationHz float64 `json:"deviation_hz"`
	DevCents    float64 `json:"dev_cents"`
}

// CompareResponses 比较修复前后频响：
// 基频偏移（音分）、各峰匹配偏差、RMS 残差与显著性。
// 显著性判据：基频偏移绝对值 > 5 音分，或峰 RMS 残差 > 0.05。
func CompareResponses(before, after *model.FrequencyResponse) *model.CompareResult {
	res := &model.CompareResult{
		ProjectID:       before.ProjectID,
		BeforeID:        before.ID,
		AfterID:         after.ID,
		PitchShiftCents: PitchShiftCents(before.BaseFreqHz, after.BaseFreqHz),
	}
	// 峰匹配：before 的每个峰在 after 中找最近峰，记录偏差。
	var shifts []PeakShift
	var devCentsSum float64
	var devCount int
	for _, p := range before.Peaks {
		matched, _, ok := MatchPeak(after.Peaks, p.FreqHz)
		if !ok {
			continue
		}
		devHz := matched.FreqHz - p.FreqHz
		dc := PitchShiftCents(p.FreqHz, matched.FreqHz)
		shifts = append(shifts, PeakShift{
			BeforeHz: p.FreqHz, AfterHz: matched.FreqHz,
			DeviationHz: devHz, DevCents: dc,
		})
		devCentsSum += dc * dc
		devCount++
	}
	if devCount > 0 {
		res.ResidualRMS = math.Sqrt(devCentsSum / float64(devCount))
	}
	res.Significant = math.Abs(res.PitchShiftCents) > 5.0 || res.ResidualRMS > 5.0
	res.Detail = fmt.Sprintf("基频偏移 %.2f 音分；峰残差 RMS %.2f 音分；%d 个谐振峰参与匹配",
		res.PitchShiftCents, res.ResidualRMS, devCount)
	return res
}

// NearestIntegerRatio 返回两个频率的最简整数比描述（用于详情展示）。
func NearestIntegerRatio(a, b float64) string {
	if a <= 0 || b <= 0 {
		return "-"
	}
	r := a / b
	for _, cand := range [][2]int{{1, 1}, {2, 1}, {3, 2}, {4, 3}, {5, 4}, {6, 5}, {9, 8}} {
		ratio := float64(cand[0]) / float64(cand[1])
		if math.Abs(r-ratio) < 0.02 {
			return fmt.Sprintf("%d:%d", cand[0], cand[1])
		}
	}
	return fmt.Sprintf("%.3f", r)
}
