# API 合约

所有JSON请求与响应使用UTF-8。成功：`{data:...,error:null,request_id:string}`。失败：`{data:null,error:{code:string,message:string},request_id:string}`。时间为UTC RFC3339，ID为UUID（上游作品ID除外）。数字点数使用整数，图片尺寸使用数字。

会话由HttpOnly、SameSite=Lax Cookie管理，默认7天，HTTPS使用Secure。写操作检查Origin。前端路由刷新由服务器返回SPA。商家归属从服务端会话读取，忽略客户端租户归属；管理员另有角色校验。

| API | 方法与数据 |
|---|---|
| /auth/login | POST email,password；email兼容字段可填写管理员用户名或商家邮箱 |
| /auth/me | GET user, nullable tenant, capabilities |
| /auth/invite | POST token,email,name,password；一次性7天邀请 |
| /auth/reset | POST token,password |
| /auth/logout | POST |
| /products | GET q,category,page（24件）；POST商品事实 |
| /products/{id} | GET、PUT商品事实、DELETE归档 |
| /products/{id}/references | POST multipart file；PUT {ids:完整顺序} |
| /products/{id}/references/{asset} | DELETE（旧任务仍保留引用） |
| /products/{id}/analyze | POST；视觉识别建议，不自动保存 |
| /presets | GET default、presets、pricing_version |
| /generations/quote | POST {product_id,preset?,topic_id?} |
| /generations | POST {quote_id,idempotency_key}；GET status,page |
| /generations/{id} | GET完整快照、文案、产物状态与版本 |
| /generations/{id}/copy | POST {copy_data,expected_version}，免费排版 |
| /generations/{id}/retry | POST {item_type}，返回报价；提交仍走/generations |
| /generations/{id}/cancel | POST；仅queued |
| /generations/{id}/selection | POST {asset_id} |
| /generations/{id}/export | POST，流式ZIP，含真实PNG、copy.json/txt、manifest.json |
| /assets/{id} | GET授权私有文件 |
| /credits、/credits/ledger | GET余额/流水，page分页 |
| /topics | GET category、product_id；共享榜单与隔离导入/匹配 |
| /topic-imports | POST {url}，返回202 id；GET /{id}查询状态 |
| /admin/overview、/tenants、/providers、/settings、/sources、/tasks | GET管理员真实数据 |
| /admin/invites、/resets、/credits | POST管理员邀请、重置、额度调整 |
| /admin/providers、/sources | POST配置；sources/{id} PUT停用/更新 |
| /admin/providers/{id}/probe | POST，仅检查连通性 |
| /admin/pricing、/limits、/collector | PUT；密钥服务端加密 |
| /admin/tasks/{id}/resolve | POST item_id,decision,reason,provider_response? |

商品必填name/category，进入生成必须有1–6张有效参考图。类目由服务端Categories规定，不默认服装。JSON可选事实为brand、external_sku、color、price（nullable）、selling_points、usage_scenarios。不以识别结果填充不确定事实。

报价保存商品与参考图、商品版本、选题、提供商配置ID、提示词/模板版本、价格版本。有效10分钟，不接受客户端价格。幂等键在商家范围内唯一，同键同报价返回已有任务；同键不同报价409。

状态：queued → running → succeeded / partial / failed / needs_review；排队任务可cancelled。子产物独立记录状态。pending/running并非可重复计费的证明：provider_attempts的submitted状态与响应文件决定能否恢复调用。

常见错误码：UNAUTHENTICATED、FORBIDDEN、NOT_FOUND、INVALID_IMAGE、FILE_TOO_LARGE、REFERENCE_LIMIT、QUOTE_EXPIRED、QUOTE_STALE、QUOTE_USED、KEY_CONFLICT、INSUFFICIENT_CREDITS、TEXT_UNCONFIGURED、IMAGE_UNCONFIGURED、COPY_CONFLICT、RENDER_PENDING、NEEDS_REVIEW。错误中文信息用于界面，稳定错误码用于逻辑。

公开合约以 internal/studio/models.go、Router与集成测试为准。引用素材ID与可选值必须匹配服务端；客户端模型不是数据库结构。
