package bore

import (
	"testing"

	"task284-boreprobe/internal/model"
)

func TestAlignSegmentsDoNotShareDiameterSlice(t *testing.T) {
	segs := []*model.BoreSegment{
		{
			ID: 1, ProjectID: 1, Label: "上节", AxialStart: 0, AxialEnd: 8,
			PitchMM: 2, Unit: "mm", DiameterMM: []float64{14.5, 14.5, 14.5, 14.5},
		},
		{
			ID: 2, ProjectID: 1, Label: "下节", AxialStart: 10, AxialEnd: 18,
			PitchMM: 2, Unit: "mm", DiameterMM: []float64{12.0, 12.0, 12.0, 12.0},
		},
	}
	ar, err := Align(segs)
	if err != nil {
		t.Fatal(err)
	}
	var upperMin, lowerMin float64 = 999, 999
	for _, pt := range ar.Bore.Points {
		if pt.SegmentLabel == "上节" && pt.DiameterMM < upperMin {
			upperMin = pt.DiameterMM
		}
		if pt.SegmentLabel == "下节" && pt.DiameterMM < lowerMin {
			lowerMin = pt.DiameterMM
		}
	}
	if upperMin < 14.0 {
		t.Fatalf("上节内径被污染: min=%.2f", upperMin)
	}
	if lowerMin > 12.5 {
		t.Fatalf("下节内径异常: min=%.2f", lowerMin)
	}
	report, err := CheckConnectivity(ar.Bore, 14.5)
	if err != nil {
		t.Fatal(err)
	}
	if report.MinDiameterMM > 12.5 {
		t.Fatalf("连通性最小内径错误: %.2f", report.MinDiameterMM)
	}
}
