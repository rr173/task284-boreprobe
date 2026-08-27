package util

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ParseID 从路径参数解析 int64 ID，非法时返回领域错误。
func ParseID(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("非法 ID: %q", raw)
	}
	return v, nil
}

// ParseFloat 解析浮点参数，非法时返回错误。
func ParseFloat(raw string, name string) (float64, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("非法 %s: %q", name, raw)
	}
	return v, nil
}

// WriteJSON 以统一信封格式输出 JSON 响应。
// 成功：{"ok":true,"data":...}；失败：{"ok":false,"error":{code,message}}。
func WriteJSON(w http.ResponseWriter, status int, ok bool, data any, errCode, errMsg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	body := map[string]any{"ok": ok}
	if ok {
		body["data"] = data
	} else {
		body["error"] = map[string]string{"code": errCode, "message": errMsg}
	}
	_ = json.NewEncoder(w).Encode(body)
}

// OK 输出 200 成功响应。
func OK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, true, data, "", "")
}

// Created 输出 201 成功响应。
func Created(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, true, data, "", "")
}

// Fail 输出错误响应，status 由调用方决定。
func Fail(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, false, nil, code, msg)
}

// MethodNotAllowed 统一 405。
func MethodNotAllowed(w http.ResponseWriter) {
	Fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "不支持该 HTTP 方法")
}

// SegmentPath 拼装路径片段，避免手工拼接转义问题。
func SegmentPath(base string, ids ...any) string {
	var sb strings.Builder
	sb.WriteString(base)
	for _, id := range ids {
		sb.WriteString("/")
		sb.WriteString(fmt.Sprint(id))
	}
	return sb.String()
}
