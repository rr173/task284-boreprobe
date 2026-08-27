package util

import (
	"testing"
)

func TestHashPartsDeterministic(t *testing.T) {
	a := HashParts("x", "y", "1.000")
	b := HashParts("x", "y", "1.000")
	if a != b {
		t.Fatal("哈希不稳定")
	}
	if a == HashParts("x", "y", "1.001") {
		t.Fatal("不同输入不应产生相同哈希")
	}
}

func TestHashFloatsNormalized(t *testing.T) {
	a := HashFloats([]float64{0.1, 14.5})
	b := HashFloats([]float64{0.1000001, 14.5000001})
	if a != b {
		t.Fatal("浮点规范化应产生相同哈希")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	orig := map[string]any{"k": 1.5, "s": "v"}
	s, err := EncodeJSON(orig)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := DecodeJSON(s, &back); err != nil {
		t.Fatal(err)
	}
	if back["s"] != "v" {
		t.Fatalf("往返不一致: %v", back)
	}
}

func TestParseID(t *testing.T) {
	if _, err := ParseID("0"); err == nil {
		t.Fatal("0 应为非法 ID")
	}
	if _, err := ParseID("abc"); err == nil {
		t.Fatal("非数字应为非法 ID")
	}
	if v, err := ParseID("42"); err != nil || v != 42 {
		t.Fatalf("解析 42 失败: %v %v", v, err)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]int{"b": 1, "a": 2, "c": 3}
	keys := SortedKeys(m)
	if len(keys) != 3 || keys[0] != "a" || keys[2] != "c" {
		t.Fatalf("排序键错误: %v", keys)
	}
}
