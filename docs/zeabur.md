# Zeabur 镜像部署

采用独立 PostgreSQL 17 服务和一个 Trend Studio 服务。平台启动命令为 `/app/trend-studio serve`，在同一容器内运行 API 与 Worker，共用私有持久化卷；网页也由该服务提供。Zeabur 不负责源码构建，镜像使用已通过 Actions 的完整 SHA 标签。

Zeabur 的服务磁盘彼此独立，不能给两个服务分别挂载 `/data` 后假设它们共享文件。本部署保持单个应用副本；普通 Linux Compose 的独立 API/Worker 方式仍可使用。

配置参考 `deploy/zeabur/app-spec.json`，替换镜像提交、私有主密钥和数据库连接。数据库采用官方 PostgreSQL 服务定义并固定 17.6；关闭外部端口转发。应用声明 web/8080 和 `/health/ready`，由 Zeabur 域名提供 HTTPS，不需要 Caddy。

卷初始化容器以 root 设置 `/data` 归属65532，主应用使用65532用户运行。`serve` 会创建私有子目录，等待数据库就绪，再执行带 PostgreSQL advisory lock 的迁移，最后启动 API 和 Worker。

管理员通过一次性 `/app/trend-studio bootstrap-admin` 命令初始化。先设置临时 `BOOTSTRAP_ADMIN_USERNAME` 与 `BOOTSTRAP_ADMIN_PASSWORD`，待服务就绪后在平台执行该命令，然后删除两项变量并重启服务。也兼容 `BOOTSTRAP_ADMIN_EMAIL`。管理员可使用用户名登录，初始密码至少6字节；商家激活与重置密码仍要求至少8字节。初始化不会向邮箱发信。不要把密码、APP_MASTER_KEY、数据库密码或 Zeabur Token 提交仓库。

应用变量 `APP_ORIGIN` 必须等于实际 HTTPS 地址且不含末尾斜杠。提供商密钥在应用管理页配置；Zeabur 部署不会自动启用商用模型，也不会携带 CI 测试提供商。

上线后检查首页与登录、管理员权限、邀请激活、真实图片上传、刷新后的图片读取、免费排版、Worker心跳以及服务重启后的持久化。收费生成需配置图片/文字接口后再验收。

当前采集服务仍采用独立部署，需要另外配置可达的采集 API 地址和身份。平台未配置采集时，商品展示仍可用。

备份时停止应用，备份 PostgreSQL、studio-data卷以及匹配的 APP_MASTER_KEY。更新只切换应用镜像，不删除服务或卷；升级前保存备份。实际项目、服务和域名记录在本地私有交付文件中。
