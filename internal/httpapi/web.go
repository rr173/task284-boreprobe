package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"task284-boreprobe/internal/model"
	"task284-boreprobe/internal/util"
)

// handleIndex 渲染内腔剖面复核页面：展示项目列表与选中项目的腔体剖面、
// 补片位置、频响偏移与裁决摘要。页面为纯服务端渲染（无前端框架）。
func (a *API) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	projects, _ := a.svc.ListProjects()
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html lang="zh"><head><meta charset="utf-8">
<title>历史木管乐器内腔修复证据复核台</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;margin:24px;color:#1f2328;background:#fafafa}
h1{font-size:20px} h2{font-size:16px}
.card{background:#fff;border:1px solid #e3e6ea;border-radius:8px;padding:14px 18px;margin:10px 0}
table{border-collapse:collapse;width:100%} td,th{border:1px solid #e3e6ea;padding:6px 10px;font-size:13px;text-align:left}
.badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:12px;background:#eef2ff;color:#3730a3}
.narrow{color:#b42318;font-weight:600} .ok{color:#067647;font-weight:600}
svg{max-width:100%}
</style></head><body>`)
	sb.WriteString(`<h1>历史木管乐器内腔修复证据复核台</h1>`)
	sb.WriteString(`<div class="card"><h2>项目列表</h2><table><tr><th>ID</th><th>名称</th><th>乐器</th><th>标称内径(mm)</th><th>状态</th><th>操作</th></tr>`)
	for _, p := range projects {
		sb.WriteString(fmt.Sprintf(`<tr><td>%d</td><td>%s</td><td>%s</td><td>%.1f</td><td><span class="badge">%s</span></td>
			<td><a href="/?project=%d">查看剖面</a></td></tr>`,
			p.ID, htmlEsc(p.Name), htmlEsc(p.InstrumentType), p.NominalBoreMM, p.Status, p.ID))
	}
	sb.WriteString(`</table></div>`)

	rawID := r.URL.Query().Get("project")
	if rawID != "" {
		if pid, err := util.ParseID(rawID); err == nil {
			a.renderProjectDetail(&sb, pid)
		}
	}
	sb.WriteString(`</body></html>`)
	_, _ = w.Write([]byte(sb.String()))
}

// renderProjectDetail 渲染单个项目的剖面与裁决视图。
func (a *API) renderProjectDetail(sb *strings.Builder, pid int64) {
	p, err := a.svc.GetProject(pid)
	if err != nil {
		sb.WriteString(`<div class="card">项目不存在</div>`)
		return
	}
	sb.WriteString(fmt.Sprintf(`<div class="card"><h2>%s（%s）状态：%s 标称内径 %.1f mm</h2>`,
		htmlEsc(p.Name), htmlEsc(p.InstrumentType), p.Status, p.NominalBoreMM))
	segments, _ := a.svc.ListSegments(pid)
	patches, _ := a.svc.ListPatches(pid)
	impacts, _ := a.svc.ListImpacts(pid)
	resps, _ := a.svc.ListResponses(pid)
	conn, _ := a.svc.LatestConnectivity(pid)
	cmp, _ := a.svc.LatestCompare(pid)

	sb.WriteString(`<h2>内腔段</h2><table><tr><th>标签</th><th>轴向区间</th><th>状态</th><th>采样数</th></tr>`)
	for _, s := range segments {
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>[%.1f, %.1f] mm</td><td>%s</td><td>%d</td></tr>`,
			htmlEsc(s.Label), s.AxialStart, s.AxialEnd, s.Status, len(s.DiameterMM)))
	}
	sb.WriteString(`</table>`)

	sb.WriteString(`<h2>腔体剖面（内径 vs 轴向）</h2>`)
	if bore, err := a.svc.BoreProfile(pid); err == nil && len(bore.Points) > 0 {
		sb.WriteString(renderBoreSVG(bore, patches, p.NominalBoreMM))
	} else {
		sb.WriteString(`<p>尚未执行对齐，请先导入内腔段并调用 /api/projects/{id}/align。</p>`)
	}

	if conn != nil {
		cls := "ok"
		if conn.NarrowRatio < 0.9 {
			cls = "narrow"
		}
		sb.WriteString(fmt.Sprintf(`<div class="card"><h2>连通性</h2><p>最小内径 %.1f mm，占标称 %.1f%%，断点 %d 处</p>
			<p class="%s">%s</p></div>`,
			conn.MinDiameterMM, conn.NarrowRatio*100, len(conn.Gaps), cls,
			connSummary(conn)))
	}
	if cmp != nil {
		sb.WriteString(fmt.Sprintf(`<div class="card"><h2>频响比较</h2><p>基频偏移 %.1f 音分（显著=%v），峰残差 RMS %.1f 音分</p></div>`,
			cmp.PitchShiftCents, cmp.Significant, cmp.ResidualRMS))
	}

	sb.WriteString(`<h2>补片与裁决</h2><table><tr><th>补片</th><th>区间</th><th>材料</th><th>裁决</th><th>证据</th></tr>`)
	for _, pt := range patches {
		kind := "未裁决"
		evidence := "-"
		for _, imp := range impacts {
			if imp.PatchID == pt.ID {
				kind = imp.Kind
				evidence = imp.Evidence
			}
		}
		sb.WriteString(fmt.Sprintf(`<tr><td>#%d</td><td>[%.1f, %.1f]</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			pt.ID, pt.AxialStart, pt.AxialEnd, htmlEsc(pt.Material), kind, htmlEsc(truncate(evidence, 80))))
	}
	sb.WriteString(`</table>`)

	sb.WriteString(fmt.Sprintf(`<div class="card"><h2>频响观测</h2><p>before %d 条，after %d 条</p></div>`,
		countKind(resps, "before"), countKind(resps, "after")))
	sb.WriteString(`</div>`)
}

// renderBoreSVG 把内腔轮廓渲染为 SVG 剖面图，补片区间以红色矩形标出。
func renderBoreSVG(bore *model.AlignedBore, patches []*model.Patch, nominal float64) string {
	pts := bore.Points
	if len(pts) == 0 {
		return `<p>无轮廓数据</p>`
	}
	const W, H = 640, 200
	minX, maxX := pts[0].AxialMM, pts[len(pts)-1].AxialMM
	minY, maxY := pts[0].DiameterMM, pts[0].DiameterMM
	for _, p := range pts {
		if p.AxialMM < minX {
			minX = p.AxialMM
		}
		if p.AxialMM > maxX {
			maxX = p.AxialMM
		}
		if p.DiameterMM < minY {
			minY = p.DiameterMM
		}
		if p.DiameterMM > maxY {
			maxY = p.DiameterMM
		}
	}
	spanX := maxX - minX
	if spanX <= 0 {
		spanX = 1
	}
	spanY := maxY - minY
	if spanY <= 0 {
		spanY = 1
	}
	px := func(x float64) float64 { return 20 + (x-minX)/spanX*(W-40) }
	py := func(y float64) float64 { return H - 20 - (y-minY)/spanY*(H-40) }
	line := func(x1, y1, x2, y2 float64, stroke string) string {
		return fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`,
			px(x1), py(y1), px(x2), py(y2), stroke)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">`, W, H))
	// 标称内径参考线
	sb.WriteString(line(minX, nominal, maxX, nominal, "#98a2b3"))
	// 轮廓折线
	prev := pts[0]
	for _, p := range pts[1:] {
		sb.WriteString(line(prev.AxialMM, prev.DiameterMM, p.AxialMM, p.DiameterMM, "#1d4ed8"))
		prev = p
	}
	// 补片矩形
	for _, pt := range patches {
		x1, y1 := px(pt.AxialStart), py(0)
		x2, y2 := px(pt.AxialEnd), py(maxY*1.02)
		sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="rgba(220,38,38,0.25)" stroke="#dc2626" stroke-dasharray="3,2"/>`,
			x1, y1, x2-x1, y2-y1))
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

func connSummary(c *model.ConnectivityReport) string {
	if c.NarrowRatio < 0.9 {
		return "结论：存在显著内径缩窄，需复核补片影响"
	}
	return "结论：管腔连通性良好，未见显著缩窄"
}

func countKind(resps []*model.FrequencyResponse, kind string) int {
	n := 0
	for _, r := range resps {
		if r.Kind == kind {
			n++
		}
	}
	return n
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func htmlEsc(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(s)
}
