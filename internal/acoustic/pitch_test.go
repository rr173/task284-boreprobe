package acoustic

import (
	"math"
	"testing"

	"task284-boreprobe/internal/model"
)

func TestPitchShiftCents(t *testing.T) {
	// 440 -> 442 = 1200*log2(442/440) ≈ 7.85 音分。
	got := PitchShiftCents(440, 442)
	if math.Abs(got-7.85) > 0.05 {
		t.Fatalf("PitchShiftCents(440,442)=%.3f, want ≈7.85", got)
	}
	if PitchShiftCents(442, 440) >= 0 {
		t.Fatalf("下行偏移应为负，实际 %v", PitchShiftCents(442, 440))
	}
}

func TestMatchPeak(t *testing.T) {
	peaks := []model.Peak{{FreqHz: 440, Amplitude: 1}, {FreqHz: 880, Amplitude: 0.6}}
	best, dev, ok := MatchPeak(peaks, 885)
	if !ok {
		t.Fatal("应匹配到峰")
	}
	if best.FreqHz != 880 {
		t.Fatalf("应匹配 880Hz，实际 %v", best.FreqHz)
	}
	if math.Abs(dev-5) > 0.001 {
		t.Fatalf("偏差应为 5Hz，实际 %v", dev)
	}
}

func TestCompareResponsesSignificant(t *testing.T) {
	before := &model.FrequencyResponse{ProjectID: 1, ID: 1, BaseFreqHz: 440,
		Peaks: []model.Peak{{FreqHz: 440, Amplitude: 1}}}
	after := &model.FrequencyResponse{ProjectID: 1, ID: 2, BaseFreqHz: 442,
		Peaks: []model.Peak{{FreqHz: 442, Amplitude: 1}}}
	res := CompareResponses(before, after)
	if !res.Significant {
		t.Fatalf("7.8 音分偏移应显著，实际 %+v", res)
	}
	if res.BeforeID != 1 || res.AfterID != 2 {
		t.Fatalf("比较结果应携带观测 ID")
	}
}

func TestCompareResponsesInsignificant(t *testing.T) {
	before := &model.FrequencyResponse{ProjectID: 1, ID: 1, BaseFreqHz: 440,
		Peaks: []model.Peak{{FreqHz: 440, Amplitude: 1}}}
	after := &model.FrequencyResponse{ProjectID: 1, ID: 2, BaseFreqHz: 440.5,
		Peaks: []model.Peak{{FreqHz: 440.5, Amplitude: 1}}}
	res := CompareResponses(before, after)
	if res.Significant {
		t.Fatalf("1.97 音分偏移不应显著，实际 %+v", res)
	}
}

func TestResolvePairIncomplete(t *testing.T) {
	only := []*model.FrequencyResponse{{ID: 1, Kind: "before"}}
	if _, _, err := ResolvePair(only); err == nil {
		t.Fatal("缺少 after 应报错")
	}
}

func TestNearestIntegerRatio(t *testing.T) {
	if r := NearestIntegerRatio(880, 440); r != "2:1" {
		t.Fatalf("880/440 应为 2:1，实际 %s", r)
	}
}
