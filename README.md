# task284-boreprobe — 历史木管乐器内腔修复证据复核台

面向乐器修复研究者的证据复核后端服务：研究者导入内腔扫描段、补片测量与修复前后频响观测，服务执行尺度校正与段对齐、管腔连通性检查、频响残差比较，并裁决补片是否造成内径缩窄或音高偏移；研究者确认影响后发布绑定测量基准的冻结快照，最终封存项目作为修复证据版本。

## 业务闭环

1. 创建修复项目（记录乐器类型与标称内径）→ 2. 导入内腔段 / 补片 / 频响观测 → 3. 执行对齐与连通性检查 → 4. 比较修复前后频响 → 5. 裁决补片影响（无影响 / 缩窄 / 响应冲突）→ 6. 发布冻结快照 → 7. 封存项目。

## 标准命令

```bash
# 构建 / 静态检查 / 测试 / 端到端自检（均须真实通过）
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/boreprobe --smoke-test

# 启动服务
go run ./cmd/boreprobe --addr :8080 --db boreprobe.db
```

## HTTP API（前缀 /api）

| 能力 | 入口 | 说明 |
|---|---|---|
| 创建项目 | POST /api/projects | 名称、乐器类型、标称内径 |
| 项目列表/详情 | GET /api/projects, /api/projects/{id} | |
| 项目状态流转 | PATCH /api/projects/{id}/status | pending→to_align→to_review→published→sealed |
| 导入内腔段 | POST /api/projects/{id}/segments | 内容哈希幂等 |
| 段状态 | PATCH /api/segments/{id} | raw/aligned/missing/anomalous/excluded |
| 记录补片 | POST /api/projects/{id}/patches | 轴向区间 + 厚度 + 材料 |
| 导入频响 | POST /api/projects/{id}/responses | before/after + 基频 + 谐振峰 |
| 对齐+连通性 | POST /api/projects/{id}/align | 输出轮廓与连通性报告 |
| 频响比较 | POST /api/projects/{id}/compare | 音分偏移 + 峰残差 RMS + 显著性 |
| 创建影响候选 | POST /api/projects/{id}/impacts | 自动越界检查与候选证据 |
| 裁决/确认影响 | PATCH /api/impacts/{id}, POST /api/impacts/{id}/confirm | 乐观锁版本校验 |
| 发布快照 | POST /api/projects/{id}/snapshots | 冻结并替代旧版 |
| 封存项目 | POST /api/projects/{id}/seal | 校验测量基准一致 |
| 统计/健康 | GET /api/projects/{id}/stats, /api/health | |

页面入口 `GET /` 渲染腔体剖面 SVG、补片位置与裁决摘要。

## 代码结构

```
cmd/boreprobe/      入口（--addr / --db / --smoke-test）
internal/model/     实体与状态机、领域错误
internal/store/     SQLite 持久化（8 类实体 CRUD + 迁移 + 事务）
internal/bore/      段对齐、尺度校正、连通性检查、剖面统计
internal/acoustic/  基频偏移（音分）、谐振峰匹配、频响比较
internal/impact/    补片影响裁决
internal/snapshot/  快照发布、冻结、替代与基准校验
internal/service/   业务编排
internal/httpapi/   HTTP 层 + 腔体剖面页面
internal/util/      幂等哈希与 JSON 助手
```

## 关键不变量

- 封存项目拒绝任何修改；冻结快照除被替代外不可变更。
- 段与频响按内容哈希幂等，重复导入返回既有记录。
- 影响裁决采用乐观锁版本校验，并发裁决只成功一次。
- 补片轴向区间必须落在已对齐轮廓覆盖范围内。
- 段轴向重叠且内径偏差超 5% 判为矛盾，拒绝对齐。
