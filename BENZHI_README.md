# BENZHI 评测说明

基于 Go 实现的文物乐器修复证据复核 Web 项目，一款后端服务，完成内腔段尺度校正对齐、管腔连通性与补片越界检查、修复前后频响残差比较与补片影响裁决，并发布绑定测量基准的冻结修复快照。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/boreprobe --addr :8080 --db boreprobe.db
```

浏览器打开 `http://localhost:8080/` 可查看腔体剖面与裁决摘要。

## 自检（不启动长驻服务）

```bash
go run ./cmd/boreprobe --smoke-test
```

自检真实创建项目、导入内腔段/补片/频响、执行对齐与连通性检查、比较频响、裁决影响、发布冻结快照并封存项目，随后关闭并重新打开数据库验证持久化与重启恢复，以退出码 0 结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
```

Go 1.26.3（GOTOOLCHAIN=local），SQLite 3.46.1（modernc.org/sqlite v1.52.0，纯 Go 驱动，CGO 无关）。

## HTTP API

路由统一以 `/api` 开头，覆盖项目、内腔段、补片、频响观测、对齐、连通性、频响比较、影响裁决、快照发布、统计与健康检查等 25+ 个端点；响应统一信封格式 `{"ok":true,"data":...}`。

## 持久化

SQLite 保存项目、段、补片、频响、连通性报告、影响裁决、比较结果与快照八类数据；段与频响按内容哈希幂等防重复导入；冻结快照绑定测量基准哈希，封存后拒绝一切修改。
