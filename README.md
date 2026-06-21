# yudao-go

> Java yudao(ruoyi-vue-pro)的 Go 语言移植 + 业务时间线 / 自用 ERP 两大原创模块。
> 单一可执行文件、毫秒级启动、~50 MB 内存,对接原版 Vue3 前端零兼容性损失。

📦 **monorepo 三块**:
- [`yudao-go/`](./yudao-go) — Go 后端(Gin + GORM + Redis Streams + WebSocket)
- [`yudao-ui-admin-vue3/`](./yudao-ui-admin-vue3) — 原版 Vue3 前端 fork,加 Axelor 风格 myerp 视图
- [`tools/`](./tools) — OpenTelemetry Java agent(BPM 混合架构用)

---

## 一、为什么把后端换成 Go

| 维度 | Java 原版(Spring Boot 3.5) | Go 移植(Gin 1.10) |
|---|---|---|
| **启动速度** | JVM 冷启动 15-30 s | 单二进制,**毫秒级** |
| **常驻内存** | 单进程 400-800 MB | **~50 MB** |
| **部署体积** | jar + JRE,镜像 200+ MB | 单 `.exe`/`.bin`,镜像 **30 MB** |
| **并发模型** | 线程池 + reactor 调优 | goroutine 原生,**IO 密集场景天然友好** |
| **编译产物** | jar 需 JVM | 静态二进制,**跨平台分发** |
| **GC 调优** | 需要(G1 / ZGC 选型 + 堆大小) | 大部分场景**零调优** |
| **冷启动延迟** | 首请求 1-3 s | <10 ms |

**真实收益**:
- 多实例部署可用 `docker compose scale` 直接横扩,**单台 4 GB 机器跑 10+ 实例无压力**
- CI/CD 流水线编译 + 镜像 push **从 5 分钟降到 30 秒**
- 本地开发改一行代码到看到效果 **2 秒**(`go build && ./bin/server.exe`),不再等 Spring context 加载

**做了哪些 Go 没原生支持的横切能力**:
- **多租户**:GORM Callback 注入 `tenant_id`,跟 Java MyBatis 拦截器等价
- **数据权限**:GORM Scopes 按角色 `data_scope` 拼 WHERE,跟 `@DataPermission AOP` 等价
- **操作日志/API 日志**:Gin 中间件,显式调用代替 `@LogRecord AOP`(无注解魔法,符合 Go 哲学)
- **链路追踪**:OpenTelemetry SDK + context 透传,跟 Java SkyWalking 等价
- **限流/幂等**:Redis 中间件,跟 `@RateLimiter`/`@Idempotent` AOP 等价
- **字段脱敏**:`mask` struct tag + `web.Success` 集中打码,`system:user:unmask` 权限可见明文

**实事求是的局限**:
- **Flowable 工作流引擎 Go 无等价**——BPM 模块仍用 Java(独立 `yudao-server.jar` 跑 :48081),Go 网关反向代理 `/admin-api/bpm/**` 过去。**混合架构,但用户无感**。

---

## 二、已完成的移植 & 新增能力

### A. system 模块(用户/权限/通知/OAuth)— **10 期全完成**

| 子模块 | 内容 |
|---|---|
| **认证授权** | 登录 / 注册 / 短信验证 / 验证码 / Token 刷新 |
| **用户管理** | 用户 / 角色 / 部门 / 岗位 / 菜单(数据权限按 `data_scope` 过滤) |
| **字典** | 字典类型 + 字典数据 |
| **多租户** | 租户 / 租户套餐(行级隔离 + 套餐权限) |
| **操作日志** | 登录日志 + 操作日志(GORM Hook 自动埋点) |
| **消息中心** | 站内信 / 邮件 / 短信 + 模板 + 发送记录 |
| **OAuth 2.0** | 应用管理 / 令牌管理(对接第三方平台) |

### B. infra 模块(基础设施)— **15 期全完成**

| 子模块 | 内容 |
|---|---|
| **配置管理** | 系统配置(可热更) |
| **定时任务** | 自研 cron 调度器 + `job:cron:*` 分布式锁去重 |
| **数据源** | 多数据源配置 |
| **文件管理** | 文件列表 / 文件配置(支持 S3/MinIO/本地) |
| **API 监控** | 访问日志 / 错误日志(中间件自动记录) |
| **基础监控** | MySQL 监控 / Redis 监控 |
| **代码生成器** | `text/template` 引擎,生成 Go(model/handler) + Vue(api/index) + 菜单 SQL,**支持单表/树表** |

### C. chatter 业务时间线模块 — **🆕 原创,Java 版没有**

> 借鉴 Odoo / Axelor Chatter 设计,统一为任意业务实体提供操作历史、字段审计、评论留言、@ 提及、文件附件、关注订阅、实时通知。

| 阶段 | 完成度 |
|---|---|
| P0 框架骨架 | 9 张表 + DDD 4 层 + Redis Streams 事件总线 ✅ |
| P1-P3 数据/事件/分布式 | 事务发件箱 + Outbox Relay + 幂等消费 ✅ |
| P4 WebSocket | Redis Pub/Sub fan-out 多实例广播 ✅ |
| P5 前端 | 通用 `<Chatter biz-type biz-id>` 组件 ✅ |
| **P5b 体验闭环** | `@` 提及自动补全(el-mention)、评论回复(Axelor 风格平铺)、订阅类型/静音、顶栏 NotificationBell ✅ |

**接入示例**(任意业务实体加一行):
```vue
<Chatter biz-type="system_user" :biz-id="userId" />
```
就能在用户详情页右侧看到完整时间线:谁改了什么字段、谁评论了什么、谁审批通过、字段 diff 高亮。

### D. myerp 自用 ERP 模块 — **🆕 原创,Java 版没有**

> 11 张表 + 38 API,完整覆盖**计量内核**这一档(SAP CWM / 金蝶浮动换算 同档)。

**核心特性**:
| 特性 | 说明 |
|---|---|
| **EAV 动态属性** | 9 种 `input_type` 校验(text/number/select/multi_select/bool/date/datetime/url/color)+ 跨分类属性继承 |
| **多单位换算(固定)** | 1 斤=50 颗、1 箱=5000 颗,产品级 `factor`,支持采购/销售包装单位 |
| **双计量 / 批次(浮动)** | Catch Weight:手工件这批 10 克/个、下批 10.2 克/个;批次实测换算率 + 出入库流水账本(`stock_move`) |
| **batch-less 模式** | 随机重量:生鲜过磅、钢板、散装、废料,每笔单据自带主辅双数量,直接落产品级 |
| **SPU/SKU 模板变体** | 借鉴 Odoo `product.template` 精简版:模板共享字段(名称/分类/单位/基础售价),`POST /template/generate-variants` 笛卡尔积一次性建 SKU,变体售价 = base + Σ `price_extra` |
| **容差校验** | 名义换算率 ± `tolerance_pct`,防录错(把 10 克录成 12 克会被拦) |
| **chatter 审计** | 产品/分类/属性/模板变更自动写时间线 |

### E. 横切能力 — **跟 Java 原版功能对齐**

- 多租户(GORM Callback)
- 审计字段自动填充(BeforeCreate / BeforeUpdate Hook)
- 数据权限(GORM Scopes)
- 链路追踪(OpenTelemetry + Jaeger,UI `:16686`)
- 操作日志 / API 日志(Gin 中间件)
- 限流(redis_rate)/ 幂等(Redis SETNX)
- 操作权限(中间件按路径推导权限码校验)
- 字段脱敏(`mask` tag + 集中打码,`system:user:unmask` 权限可见明文)
- 角色操作审计入 chatter(用户分配角色的变更显示在用户详情页"动态"抽屉)

### F. BPM 工作流 — **混合架构(用户无感)**

- Flowable 引擎无 Go 等价,故 `yudao-server.jar`(裁剪为 framework+system+infra+bpm)作为独立 Java 服务跑 `:48081`
- 与 Go 共享 `yudao_go` 数据库
- Go 网关反向代理 `/admin-api/bpm/**` → :48081
- 前端工作流菜单可用,前端代码完全不知道下游是 Java 还是 Go

### G. 前端(yudao-ui-admin-vue3)— **fork 自上游 + 加 Axelor 风格组件库**

- 原版 yudao-ui-admin-vue3 fork(Vue3 + Element Plus + TypeScript + pnpm + Vite)
- **新增**:Axelor 风格通用组件(`src/components/AxelorStyle/AxelorGrid` + `AxelorDetail`)
  - 列内联搜索 + 列管理 + 上下条记录导航 + 更多▾ 下拉 + 复制为新
- **新增**:myerp 自用 ERP 全套视图(分类树 / 属性 / 产品 / 单位 / 批次 / 模板),走 Axelor List-Detail 范式
- **新增**:Chatter 组件,任意业务实体一行接入
- 跟原版 Dialog 风格**零冲突共存**

---

## 三、技术栈

```
后端 (yudao-go)      Go 1.22 · Gin · GORM · MySQL · Redis · Redis Streams · WebSocket · OpenTelemetry
前端 (vue3)          Vue3 + <script setup> · Element Plus · TypeScript · pnpm · Vite
BPM (混合架构)        Java 17 · Spring Boot 3.5 · Flowable · 反向代理至 :48081
依赖容器             Docker Compose(MySQL :13306 · Redis :16381 · Jaeger :16686)
```

---

## 四、架构总览

```
                     ┌─────────────────────────────────┐
                     │   yudao-ui-admin-vue3 (:5174)   │
                     │   Vue3 + Element Plus           │
                     └────────────┬────────────────────┘
                                  │ HTTP + WebSocket
                                  ▼
        ┌──────────────────────────────────────────────────────────┐
        │           Gin Gateway (yudao-go :48090)                  │
        │  认证 → 租户 → 数据权限 → 限流 → 幂等 → 操作权限 → 审计       │
        └──┬───────────┬─────────────┬────────────┬────────┬───────┘
           │           │             │            │        │
        system      infra        chatter        myerp      代理 ─→ Java BPM (:48081)
           │           │             │            │                  │
           └───────────┴──────┬──────┴────────────┘                  │
                              │                                      │
                ┌─────────────▼──────────────┐                       │
                │  MySQL :13306 / Redis :16381 │ ◄────共享 yudao_go───┘
                └──────────────┬───────────────┘
                               │ otel
                               ▼
                        Jaeger :16686 (链路追踪)
```

---

## 五、快速开始

```bash
# 1. 起依赖容器
cd yudao-go
docker compose -f deploy/docker-compose.yml up -d

# 2. 起后端(端口 48090)
go build -o bin/server.exe ./cmd/server && ./bin/server.exe

# 3. 起前端(端口 5174,账号 admin/admin123)
cd ../yudao-ui-admin-vue3
pnpm install && pnpm run dev

# 4.(可选)起 BPM Java 服务
#    详见 yudao-go/CLAUDE.md 中 BPM 启动小节
```

访问 `http://localhost:5174` → 用 `admin/admin123` 登录。

---

## 六、目录索引

- 后端架构详解 → [`yudao-go/CLAUDE.md`](./yudao-go/CLAUDE.md)
- chatter 模块设计 → 后端代码 `internal/module/chatter/`
- myerp 模块设计 → 后端代码 `internal/module/myerp/`
- Axelor 风格组件库 → 前端代码 `src/components/AxelorStyle/`

