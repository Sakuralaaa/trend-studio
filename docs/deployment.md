# 部署、备份与恢复

所有下列 Docker 操作在 Linux 服务器执行；用户本机只读取/编辑文件和进行 Git 操作。平台与采集服务是两个独立 Compose 项目。

## 平台首次部署

1. 下载仓库部署目录，进入 `deploy`，复制 `.env.example` 为私有 `.env`。设置域名、APP_ORIGIN（无末尾斜杠）、数据库密码和 DATABASE_URL。
2. 服务器执行 `openssl rand -base64 32` 生成 APP_MASTER_KEY。它是 AES-GCM 主密钥，必须与数据库备份一起保存，不能提交仓库。
3. 设置 TREND_IMAGE 为已通过 Actions 的完整 SHA 标签或镜像摘要。不要使用尚未验证的提交或自制镜像。
4. 如 GHCR 包为私有，使用有 `read:packages` 权限的 GitHub PAT 登录。避免将 PAT 放进命令历史：`printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u YOUR_USER --password-stdin`。包可见性由 GitHub Packages 实际设置决定，公开仓库不等于公开包。
5. 执行 `docker compose --env-file .env pull`，再执行 `docker compose --env-file .env up -d`。存储初始化以 root 执行一次，将目录归属设置为65532；随后迁移和 API/Worker 均以非 root 执行。Caddy负责TLS。数据库不发布端口。
6. `docker compose ps` 确认迁移退出0、API健康。浏览器访问你的域名。

## 初始化管理员

服务器以交互输入环境变量后执行一次初始化，密码不会写进仓库：

```sh
read -r BOOTSTRAP_ADMIN_EMAIL
read -rs BOOTSTRAP_ADMIN_PASSWORD; printf '\n'
export BOOTSTRAP_ADMIN_EMAIL BOOTSTRAP_ADMIN_PASSWORD
docker compose run --rm -e BOOTSTRAP_ADMIN_EMAIL -e BOOTSTRAP_ADMIN_PASSWORD api bootstrap-admin
unset BOOTSTRAP_ADMIN_EMAIL BOOTSTRAP_ADMIN_PASSWORD
```

已有管理员时初始化拒绝执行。管理员没有商家空间或虚构余额。进入管理平台，配置提供商和价格，创建7天有效邀请，将一次性链接交给目标商家。首版不提供在线支付。重置链接由管理员生成，1小时有效，使用后撤销旧会话。

## 提供商设置

基础地址示例 `https://provider.example/v1`，服务拼接 `/chat/completions` 或 `/images/edits`。图片接口每次请求1张，multipart发送真实参考图。多参考图字段为 `image[]`，单图接口为 `image`；最大数量、输出尺寸、超时必须与服务实际能力一致。无图像编辑能力时不可配置为图片通道。

接口连接检测只检查 `/models`，不意味着图片编辑或计费已经验证。视觉识别只有文字通道的视觉模型字段非空时出现；识别结果由商家确认后保存。请先使用自己的公开测试商品进行真实接口验收。

结果图片支持 b64_json 或 URL。URL下载限制协议、IP、重定向和可选域名白名单；不转发提供商认证头。`ALLOW_PRIVATE_PROVIDER=true` 仅为 CI 连接测试网络使用，生产不应开启。

## 独立采集服务

进入 `deploy/collector`，复制示例为 `.env`。采用 upstream v5.1.3 入口命令 `migrate / api / worker`，不是旧版 FastAPI/Celery 命令。配置独立 DTK_SECRET_KEY、Postgres 和 Redis 密码、DTK_DATABASE_URL 与 DTK_REDIS_URL。固定镜像 `evil0ctal/douyin_tiktok_download_api:5.1.3`，TimescaleDB PG17 与 Redis8，私有卷互不混用。

执行 `docker compose --env-file .env pull` 和 `docker compose --env-file .env up -d`。API仅绑定主机回环8000端口。通过SSH转发或服务器自己的访问方式打开 upstream 控制台，按照其首次启动 setup token 流程初始化管理员，配置合法的采集身份/账号与 API Key。API Key由上游控制台创建，不是随意设置的 API_KEY 环境变量。

平台容器不能用 `localhost:8000` 访问另一容器。使用服务器私有地址/内网反向代理，或给两个 API 服务附加专用外部 Docker 网络（不共享数据库网络），平台配置采集地址为可达的采集 API 服务名。不要直接开放无TLS的8000端口到公网。

在 Trend Studio 管理页填写采集地址/API Key并维护行业账号。缺少身份或反爬阻断时显示错误与更新时间；不影响商品展示。可选浏览器RPC没有上游预构建镜像，本项目默认不启用。

## 备份

备份数据库、整个 studio-data 卷以及 APP_MASTER_KEY，三者属于同一恢复点。先暂停 Worker、等待执行中的提供商调用完成或标记为待核实，再停止 API，防止数据库与文件备份时间错位。

```sh
docker compose stop worker api
docker compose exec -T db pg_dump -U studio -Fc studio > studio-db.dump
docker run --rm --volumes-from "$(docker compose ps -a -q api)" -v "$PWD:/backup" alpine:3.22 tar czf /backup/studio-data.tar.gz -C /data .
docker compose start api worker
```

加密存放备份与私有.env；采集数据库、卷和DTK_SECRET_KEY单独备份。定期做恢复演练。不要执行 `down -v` 清理生产。

## 升级与恢复

升级前记录旧镜像摘要并备份。修改 TREND_IMAGE 为新的已验证SHA，拉取，停止 Worker，执行迁移后更新 API/Worker。迁移目前为可重入的初始版本；后续破坏性迁移须另设版本并测试，不以旧镜像回滚代替数据恢复。

恢复时先还原数据库、素材卷和匹配主密钥，再启动API/Worker。运行中的持久化任务按租约恢复；已保存提供商响应继续解析，已发送但无保存响应的调用进入待核实。不要直接将该调用改为待执行。

管理员核实未知单项时：供应商明确失败可退还；供应商返回了真实JSON可导入响应并继续处理。填写核实依据，保留审计记录。不能用按钮强行重复扣点。

## 日志与健康

API `/health/live` 检查进程；`/health/ready` 检查数据库与存储可写。Worker每15秒写心跳。管理员概览读取数据库实际指标，成本缺少可靠数据时显示暂无数据。服务器应监控卷容量、数据库备份、Worker心跳和待核实任务。
