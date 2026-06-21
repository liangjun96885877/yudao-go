# yudao-go

将 Java 版 yudao（ruoyi-vue-pro / yudao-boot-mini）移植为 Go 项目，并新增 **chatter 业务时间线**模块。
对接**原版前端** `yudao-ui-admin-vue3`（位于 `../yudao-ui-admin-vue3`）。

## 技术栈
Go 1.22 · Gin · GORM · MySQL · Redis · Redis Streams(事件总线) · WebSocket · 原版 Vue3 + Element Plus 前端。

## 运行方式

```bash
# 1. 依赖容器（项目名 yudaogo-dev：MySQL 13306 / Redis 16381 / Jaeger 16686，独立端口）
docker compose -f deploy/docker-compose.yml up -d

# 2. 后端（端口 48090；48080 被原版 Java 服务占用）
go build -o bin/server.exe ./cmd/server && ./bin/server.exe

# 3. 前端（原版 yudao 后台，端口 5174，账号 admin/admin123）
cd ../yudao-ui-admin-vue3 && pnpm run dev

# 4. BPM 工作流 Java 服务（方案 A：端口 48081，与 Go 共享 yudao_go 库）
#    BPM 基于 Flowable 引擎，无 Go 等价实现，故以独立 Java 服务运行，Go 网关反向代理 /admin-api/bpm/**
#    须挂 OTel agent 以接入链路追踪（PowerShell，Windows 路径）：
cd ../ruoyi-vue-pro
# javaagent 路径相对 ruoyi-vue-pro 当前目录;tools/ 与 yudao-go / ruoyi-vue-pro 同级
java "-javaagent:../tools/opentelemetry-javaagent.jar" `
  "-Dotel.service.name=yudao-server-bpm" "-Dotel.exporter.otlp.endpoint=http://127.0.0.1:14318" `
  "-Dotel.exporter.otlp.protocol=http/protobuf" "-Dotel.metrics.exporter=none" `
  "-Dotel.logs.exporter=none" -jar yudao-server\target\yudao-server.jar
#    构建（IntelliJ 内置 Maven）：mvn clean package -pl yudao-server -am -DskipTests
```

链路追踪：Jaeger all-in-one 由 docker-compose 启动；UI `http://localhost:16686`。
Go 后端经 `OTEL_ENDPOINT`（默认 `127.0.0.1:14318`）、`OTEL_ENABLED` 控制。

数据库 `yudao_go`：已导入原版 `sql/mysql/ruoyi-vue-pro.sql`（48 表 + 种子数据）+ chatter 9 张表
（`deploy/sql/chatter_schema.sql`）。

## 目录结构
- `internal/framework/` — web / orm / redisx / eventbus / websocket / security / config / logger / contextx
- `internal/module/chatter/` — 业务时间线模块（DDD 分层：domain/application/infrastructure/interfaces）
- `internal/module/system/` — 系统管理模块（model / repo / service / rest）
- `cmd/server/main.go` — 进程入口
- `docs/移植能力替换标准.md` — 移植规范（强制）
- `docs/已实现能力清单.md` — 已实现能力登记表，**新增能力前先查此表**

## 进度
- chatter：P0 框架骨架 → P5 前端 → **P5b 体验闭环**,全部完成。
  P5b 含:@ 提及自动补全(el-mention)、评论回复(Axelor 风格 · 平铺 + 按需展开,递归任意深度)、
  订阅类型/静音设置、顶栏 NotificationBell 接入(登录即建 WS 长连 + 点通知跳对应记录动态)。
- system 迁移：S1 登录引导 → 消息中心 → S10 OAuth 2.0 完成。系统管理菜单下
  用户/角色/菜单/部门/岗位/字典/租户/租户套餐/登录日志/操作日志、
  消息中心（站内信/邮件/短信，含模板与发送记录）、
  OAuth 2.0（应用管理/令牌管理）均可用。
- infra 迁移：S5 配置管理 → S15 代码生成器 完成。基础设施菜单下
  配置管理/定时任务/数据源配置/文件列表/文件配置/API 访问日志/错误日志、
  监控中心（MySQL 监控/Redis 监控）、代码生成 均可用；
  `framework/job` 调度器运行中；API 访问日志中间件记录所有请求。
- 代码生成器：`text/template` 引擎，模板在 `internal/module/infra/rest/codegen_tpl/`，
  生成 Go(model/handler) + Vue(api/index) + 菜单 SQL，支持单表/树表。
- 横切能力：多租户 / 审计填充 / 数据权限（GORM 回调）、链路追踪（otel）、
  操作日志 / API 日志 / 限流 / 幂等 / 操作权限 / 脱敏豁免（中间件）。
  数据权限按角色 data_scope 过滤含 dept_id 的表;操作权限按路径推导权限码校验;
  数据脱敏经 `mask` tag + `web.Success` 集中打码,`system:user:unmask` 权限可见明文。
- 角色操作审计入 chatter:角色基础信息/菜单权限/数据范围/字段权限的变更已接入 `system_role` 时间线;
  用户分配角色变更接入 `system_user` 时间线(diff 出"角色: 旧名 → 新名"),用户页"动态"抽屉可直接看到。
- chatter 已集成进原版前端（侧边栏「业务时间线」菜单 + 用户管理页「动态」按钮抽屉）。
- 自用 ERP（myerp）：参考一个 go-zero 微服务原型移植并重新设计的 EAV 动态属性模型 ERP（分类/属性/产品/单位/批次/模板），
  后端 DDD 4 层 `internal/module/myerp/`（11 表 + 38 API + 9 种 input_type 校验 + chatter 审计 +
  企业级多单位换算 + 双计量/批次），前端用 **Axelor 风格 List-Detail**（非原版 Dialog），两套范式零冲突共存。
  多单位换算解决「螺丝一斤 50 颗」固定换算：换算率存产品级（`myerp_product_uom.factor`）。
  双计量/批次（Catch Weight）解决「同产品因批次换算率不同」（手工件这批 10 克/个、下批 10.2 克/个）：
  产品 `uom_mode` 开关（固定/浮动），浮动产品库存为主+辅两列独立结存（`stock`+`stock_aux`），
  批次 `myerp_product_batch.actual_factor` 存每批实测率，出入库经 `myerp_stock_move` 账本（库存=流水之和），
  名义率 `nominal_factor`+容差 `tolerance_pct` 做录入校验。单位字典 `myerp_uom` 被产品引用时禁删。
  浮动产品再有第二档开关 `batch_tracked`：开（默认）走批次；关 = batch-less / 随机重量
  （生鲜过磅、钢板、散装、废料），每笔单据自带主辅双数量+实际率,直接落产品级,不走批次。
  SPU/SKU 模板变体（借鉴 Odoo product.template 精简版）：`myerp_product_template` 共享字段（名称/分类/单位/基础售价），
  `myerp_product.template_id` 挂模板=变体、null=独立 SKU；`attribute.is_variant` 标记区分属性,
  `attribute_value_option.price_extra` 加价；`POST /template/generate-variants` 笛卡尔积一次性建 SKU,
  变体售价 = 模板 base_price + Σ 所选选项 price_extra。
  通用组件 `src/components/AxelorStyle/`（AxelorGrid + AxelorDetail）可复用到其它新模块。
  详见 `docs/Axelor风格组件库.md`、`docs/自用ERP-myerp.md`。
- BPM 工作流：**方案 A（混合架构）**。Flowable 引擎无 Go 等价实现，故 `ruoyi-vue-pro`
  的 `yudao-server`（裁剪为 framework+system+infra+bpm）作为独立 Java 服务运行在 48081，
  与 Go 共享 `yudao_go` 库；Go 网关把 `/admin-api/bpm/**` 反向代理过去。前端工作流菜单可用。

## 注意事项
- **改后端代码后**：`Stop-Process -Name server` → 重新 `go build` → 重启 `server.exe`。
- **前端**：vite dev server 热更新，新增文件自动生效。
- **Git Bash 中文坑**：内联中文参数（`curl -d '{中文}'`）会被 Git Bash 弄坏 UTF-8。
  测试带中文请求体用文件（`curl --data @file.json`），URL 参数用预编码。
- **新增菜单**：插入 `system_menu` 表；顶级菜单（parent_id=0）的 `path` 必须以 `/` 开头。
- **bit(1) 列**：用 `orm.Bit` 类型映射（原版用 bit(1) 表示布尔与逻辑删除）。
- **数据库**：需要新库时在 Docker 新建独立项目，不要影响现有容器。
- **BPM 服务**：改 BPM 相关代码/配置后需重新 `mvn package` 并重启 `yudao-server.jar`。
  bpm 的 8 张业务表 DDL 见 `../ruoyi-vue-pro/sql/bpm_tables.sql`（含 `tenant_id`，
  yudao 租户拦截器要求）；Flowable 的 `ACT_*` 表首次启动自动创建。
- **token user_type**：yudao 约定 会员=1 / 管理员=2。Go 登录签发的 token 必须 `user_type=2`，
  否则 BPM Java 服务按匿名处理返回 403。
- **多进程部署**：Go 移植版已多实例就绪（共享 MySQL+Redis、eventbus 配 `redis`）。
  定时任务调度器用 `job:cron:*` 分布式锁去重，每次触发仅一个实例执行。
