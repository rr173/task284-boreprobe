package impact

import (
	"testing"

	"task284-boreprobe/internal/model"
)

func TestAdjudicateNarrowed(t *testing.T) {
	dec, err := Adjudicate(&Adjudication{
		Patch: &model.Patch{ID: 1, Material: "枫木", AxialStart: 70, AxialEnd: 90},
		Connectivity: &model.ConnectivityReport{NarrowRatio: 0.86},
		Compare:      &model.CompareResult{PitchShiftCents: 7.8, Significant: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Kind != model.ImpactNarrowed {
		t.Fatalf("缩窄优先应判定 narrowed，实际 %s", dec.Kind)
	}
	if dec.Evidence == "" {
		t.Fatal("证据文本不能为空")
	}
}

func TestAdjudicateConflict(t *testing.T) {
	dec, err := Adjudicate(&Adjudication{
		Patch:        &model.Patch{ID: 2, Material: "枫木"},
		Connectivity: &model.ConnectivityReport{NarrowRatio: 0.98},
		Compare:      &model.CompareResult{PitchShiftCents: 7.8, Significant: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Kind != model.ImpactConflict {
		t.Fatalf("未缩窄但频响显著应判定 conflict，实际 %s", dec.Kind)
	}
}

func TestAdjudicateNone(t *testing.T) {
	dec, err := Adjudicate(&Adjudication{
		Patch:        &model.Patch{ID: 3, Material: "枫木"},
		Connectivity: &model.ConnectivityReport{NarrowRatio: 0.98},
		Compare:      &model.CompareResult{PitchShiftCents: 1.2, Significant: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Kind != model.ImpactNone {
		t.Fatalf("无缩窄且不显著应判定 none，实际 %s", dec.Kind)
	}
}

func TestAdjudicateMissingCompare(t *testing.T) {
	dec, err := Adjudicate(&Adjudication{
		Patch:        &model.Patch{ID: 4, Material: "枫木"},
		Connectivity: &model.ConnectivityReport{NarrowRatio: 0.98},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Kind != model.ImpactNone {
		t.Fatalf("无比较数据时应判定 none，实际 %s", dec.Kind)
	}
}
