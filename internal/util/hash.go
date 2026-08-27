// Package util 提供跨模块复用的哈希与 JSON 工具。
package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// HashParts 对任意字符串片段做稳定拼接后计算 SHA-256。
// 用于段摘要、响应摘要与测量基准哈希，保证幂等与防重复导入。
func HashParts(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0x1f})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// HashFloats 把浮点序列规范化为固定精度字符串参与哈希，
// 避免 0.1+0.2 之类的浮点噪声破坏幂等性。
func HashFloats(vals []float64) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.FormatFloat(v, 'f', 3, 64)
	}
	return strings.Join(parts, ",")
}

// EncodeJSON 把任意值编码为紧凑 JSON 字符串。
func EncodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeJSON 解析 JSON 字符串到目标结构。
func DecodeJSON(s string, v any) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("空载荷")
	}
	return json.Unmarshal([]byte(s), v)
}

// SortedKeys 返回 map 的排序键列表，保证哈希输入顺序稳定。
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
