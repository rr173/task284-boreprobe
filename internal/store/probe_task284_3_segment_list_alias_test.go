package store

import (
	"testing"

	"task284-boreprobe/internal/model"
)

func TestListSegmentsReturnsIndependentSnapshots(t *testing.T) {
	db := setupDB(t)
	p, _ := db.CreateProject("列表别名", "oboe", 14.5)
	seg := &model.BoreSegment{
		ProjectID: p.ID, Label: "原始标签", AxialStart: 0, AxialEnd: 10,
		PitchMM: 2, Unit: "mm", DiameterMM: []float64{14, 14, 14, 14, 14},
	}
	if _, err := db.CreateSegment(seg); err != nil {
		t.Fatal(err)
	}
	first, err := db.ListSegments(p.ID)
	if err != nil || len(first) != 1 {
		t.Fatalf("first list: %v %v", first, err)
	}
	second, err := db.ListSegments(p.ID)
	if err != nil || len(second) != 1 {
		t.Fatalf("second list: %v %v", second, err)
	}
	first[0].Label = "被篡改"
	if second[0].Label != "原始标签" {
		t.Fatalf("列表结果别名污染: second=%q", second[0].Label)
	}
}
