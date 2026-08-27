package bore

import (
	"testing"

	"task284-boreprobe/internal/model"
)

func seg(label string, start, end, pitch float64, unit string, diam []float64) *model.BoreSegment {
	return &model.BoreSegment{
		Label: label, AxialStart: start, AxialEnd: end,
		PitchMM: pitch, Unit: unit, DiameterMM: diam, Status: model.SegmentRaw,
	}
}

func TestScaleSegmentCM(t *testing.T) {
	s := seg("cm段", 0, 10, 1, "cm", []float64{14.5, 14.4})
	st, en, p, err := ScaleSegment(s)
	if err != nil {
		t.Fatal(err)
	}
	if st != 0 || en != 100 || p != 10 {
		t.Fatalf("cm 换算错误: %v %v %v", st, en, p)
	}
}

func TestScaleSegmentUnknownUnit(t *testing.T) {
	s := seg("未知单位", 0, 10, 1, "inch", []float64{1})
	if _, _, _, err := ScaleSegment(s); err == nil {
		t.Fatal("未知单位应报错")
	}
}

func TestAlignJoinsSegments(t *testing.T) {
	segs := []*model.BoreSegment{
		seg("上节", 0, 6, 2, "mm", []float64{14.5, 14.5, 14.4}),
		seg("下节", 6, 12, 2, "mm", []float64{14.5, 14.5, 14.4}),
	}
	ar, err := Align(segs)
	if err != nil {
		t.Fatal(err)
	}
	if len(ar.Bore.Points) != 6 {
		t.Fatalf("拼接后应为 6 个点，实际 %d", len(ar.Bore.Points))
	}
	if len(ar.Bore.SegmentIDs) != 2 {
		t.Fatalf("应有 2 个段参与，实际 %d", len(ar.Bore.SegmentIDs))
	}
}

func TestAlignExcludesSegments(t *testing.T) {
	excluded := seg("废段", 100, 106, 2, "mm", []float64{1, 1, 1})
	excluded.Status = model.SegmentExcluded
	segs := []*model.BoreSegment{
		seg("主段", 0, 6, 2, "mm", []float64{14.5, 14.5, 14.4}),
		excluded,
	}
	ar, err := Align(segs)
	if err != nil {
		t.Fatal(err)
	}
	if len(ar.Excluded) != 1 || ar.Excluded[0] != "废段" {
		t.Fatalf("应排除废段，实际 %v", ar.Excluded)
	}
	if len(ar.Bore.Points) != 3 {
		t.Fatalf("排除后应为 3 个点，实际 %d", len(ar.Bore.Points))
	}
}

func TestAlignOverlapContradict(t *testing.T) {
	segs := []*model.BoreSegment{
		seg("A", 0, 10, 2, "mm", []float64{14.5, 14.5, 14.5, 14.5, 14.5}),
		seg("B", 6, 16, 2, "mm", []float64{11.0, 11.0, 11.0, 11.0, 11.0}),
	}
	if _, err := Align(segs); err == nil {
		t.Fatal("重叠且内径矛盾应报错")
	}
}

func TestCheckConnectivityFindsDrop(t *testing.T) {
	bore := &model.AlignedBore{ProjectID: 1, Points: []model.BorePoint{
		{AxialMM: 0, DiameterMM: 14.5},
		{AxialMM: 2, DiameterMM: 12.5}, // 下降 13.8%，未到 25% 阈值
		{AxialMM: 4, DiameterMM: 14.5},
	}}
	rep, err := CheckConnectivity(bore, 14.5)
	if err != nil {
		t.Fatal(err)
	}
	if rep.MinDiameterMM != 12.5 {
		t.Fatalf("最小内径应为 12.5，实际 %v", rep.MinDiameterMM)
	}
	if rep.NarrowRatio < 0.86 || rep.NarrowRatio > 0.87 {
		t.Fatalf("缩窄比应为 ≈0.862，实际 %v", rep.NarrowRatio)
	}
}

func TestCheckConnectivityDetectsGap(t *testing.T) {
	// 中位数参考间距为 2mm，20mm 的间距超过 3 倍参考间距 → missing_scan。
	bore := &model.AlignedBore{ProjectID: 1, Points: []model.BorePoint{
		{AxialMM: 0, DiameterMM: 14.5},
		{AxialMM: 2, DiameterMM: 14.5},
		{AxialMM: 22, DiameterMM: 14.5},
		{AxialMM: 24, DiameterMM: 14.5},
	}}
	rep, err := CheckConnectivity(bore, 14.5)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range rep.Gaps {
		if g.Reason == "missing_scan(轴向间距异常)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应检测到 missing_scan 断点，实际 %+v", rep.Gaps)
	}
}

func TestPatchInRange(t *testing.T) {
	bore := &model.AlignedBore{Points: []model.BorePoint{
		{AxialMM: 0, DiameterMM: 14.5},
		{AxialMM: 100, DiameterMM: 14.5},
	}}
	if err := PatchInRange(bore, &model.Patch{AxialStart: 10, AxialEnd: 50}); err != nil {
		t.Fatalf("范围内补片不应报错: %v", err)
	}
	if err := PatchInRange(bore, &model.Patch{AxialStart: 10, AxialEnd: 150}); err == nil {
		t.Fatal("越界补片应报错")
	}
}

func TestAnalyzeProfileNarrowZone(t *testing.T) {
	bore := &model.AlignedBore{Points: []model.BorePoint{
		{AxialMM: 0, DiameterMM: 14.5},
		{AxialMM: 2, DiameterMM: 12.5},
		{AxialMM: 4, DiameterMM: 14.5},
	}}
	st := AnalyzeProfile(bore, 14.5, 0.9)
	if len(st.NarrowZones) != 1 {
		t.Fatalf("应检测到 1 个缩窄区，实际 %d", len(st.NarrowZones))
	}
	if st.NarrowZones[0].MinRatio < 0.86 || st.NarrowZones[0].MinRatio > 0.87 {
		t.Fatalf("缩窄区最小占比应 ≈0.862，实际 %v", st.NarrowZones[0].MinRatio)
	}
}
