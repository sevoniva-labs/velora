# Velora 生产运维

本文件是运维入口，不复制底层命令细节。

| 任务 | 权威文档 |
|---|---|
| 生产部署、配置和迁移 | [`server/docs/deployment.md`](../server/docs/deployment.md) |
| 日常巡检、故障处理和恢复 | [`server/docs/operations.md`](../server/docs/operations.md) |
| 安全配置和密钥边界 | [`server/docs/security.md`](../server/docs/security.md) |
| 验证命令和证据边界 | [`server/docs/validation.md`](../server/docs/validation.md) |
| S3/COS/MinIO 能力契约 | [`server/docs/storage-contract.md`](../server/docs/storage-contract.md) |
| 前端构建和浏览器测试 | [`server/docs/frontend.md`](../server/docs/frontend.md) |

## 发布顺序

1. 确认 `main` 与远程一致，工作区只有明确允许的文件。
2. 备份数据库、生产环境文件和当前镜像摘要。
3. 使用国内 Go、npm 和容器镜像源执行 `make verify` 与 `make check-prod-config`。
4. 从同一提交构建 Server、Worker、Web 和 `velora-connect`，记录版本与 SHA-256。
5. 先更新迁移和后端，再更新 Worker 与 Web。
6. 等待所有容器健康，验证 `/api/v1/system/health` 和 `/api/v1/system/ready`。
7. 验证登录、无权限空态、应用启动、审批、账号下发、目录接口、停用和退出。
8. 保存验收证据和回滚点后才结束发布。

## 回滚原则

- 回滚使用发布前保存的 Compose、环境文件、镜像标签和数据库备份。
- 先停止产生新外部事件的 Worker，再回滚应用镜像。
- 数据库迁移默认向后兼容；事故窗口不删除新表或新列。
- 凭据已经轮换时不得恢复旧明文，使用新版本重新交付。
- 回滚后重新执行健康、登录、权限拒绝、账号下发和目录接口验收。

## 当前生产目标

- Portal：`https://home.sevoniva.com`
- Identity protocol：`https://auth.sevoniva.com`
- Reference application：`https://demo.sevoniva.com`
- Production host：`ubuntu@175.27.250.53`

生产密钥、账号密码、数据库 DSN 和云厂商凭据不得写入本文档或 Git。
