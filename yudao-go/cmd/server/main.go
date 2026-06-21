// Command server 是 yudao-go 的进程入口：装配各框架组件并启动 HTTP 服务。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yudao-go/internal/framework/config"
	"yudao-go/internal/framework/eventbus"
	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/orm"
	"yudao-go/internal/framework/redisx"
	"yudao-go/internal/framework/security"
	"yudao-go/internal/framework/tracing"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/framework/websocket"
	"yudao-go/internal/module/chatter"
	"yudao-go/internal/module/chatter/registry"
	"yudao-go/internal/module/myerp"
	"yudao-go/internal/module/infra"
	"yudao-go/internal/module/system"
)

func main() {
	if err := run(); err != nil {
		logger.L().Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. 配置与日志
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger.Init(!cfg.App.IsProd())
	log := logger.L()

	// 1.5 链路追踪（须在 DB/Redis/HTTP instrumentation 使用前初始化全局 TracerProvider）
	otelEndpoint := os.Getenv("OTEL_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "127.0.0.1:14318"
	}
	otelEnabled := os.Getenv("OTEL_ENABLED") != "false"
	tracer, err := tracing.Init(context.Background(), "yudao-go", otelEndpoint, otelEnabled)
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tracer.Shutdown(ctx)
	}()
	log.Info("tracing initialized", "enabled", otelEnabled, "endpoint", otelEndpoint)

	// 2. 数据库
	db, err := orm.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	if err := orm.RegisterPlugins(db); err != nil {
		return fmt.Errorf("register orm plugins: %w", err)
	}
	txMgr := orm.NewTxManager(db)
	systemModule := system.New(txMgr)
	log.Info("database connected")

	// 3. Redis（须在 infra 模块之前创建：定时任务调度器需要它做分布式锁）
	rdb, err := redisx.New(cfg.Redis)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer func() { _ = rdb.Close() }()
	log.Info("redis connected")

	infraModule := infra.New(txMgr, rdb)

	// WebSocket Hub（多实例经 Redis Pub/Sub fan-out）
	hub := websocket.NewHub(rdb.Raw())
	hub.Start()

	// 4. 事件总线与业务模块（按配置选择进程内 / Redis Streams）
	codec := eventbus.NewCodec()
	var bus eventbus.Bus
	switch cfg.EventBus.Type {
	case "redis":
		host, _ := os.Hostname()
		bus = eventbus.NewRedisStreamBus(rdb.Raw(), codec, "chatter",
			fmt.Sprintf("%s-%d", host, os.Getpid()))
	default:
		bus = eventbus.NewInProcBus(cfg.EventBus.Workers, cfg.EventBus.BufferSize)
	}

	chatterModule := chatter.New(txMgr, bus, codec, hub)
	myerpModule := myerp.New(txMgr)
	// 注册 system_user 业务类型并接入字段审计：用户修改将自动生成 chatter 时间线。
	chatterModule.Registry.Register(registry.BizType{
		Type: "system_user", DisplayName: "用户", Table: "system_users",
		AuditFields: []registry.AuditField{
			{Column: "nickname", Field: "Nickname", Label: "昵称", Type: "string"},
			{Column: "dept_id", Field: "DeptID", Label: "部门", Type: "ref"},
			{Column: "post_ids", Field: "PostIDs", Label: "岗位", Type: "ref"},
			{Column: "email", Field: "Email", Label: "邮箱", Type: "string"},
			{Column: "mobile", Field: "Mobile", Label: "手机号", Type: "string"},
			{Column: "sex", Field: "Sex", Label: "性别", Type: "enum"},
			{Column: "remark", Field: "Remark", Label: "备注", Type: "string"},
			{Column: "roles", Field: "Roles", Label: "角色", Type: "ref"},
		},
	})
	// 角色:把 增/改/删、菜单权限、数据权限、字段权限 的变更都计入角色的动态时间线。
	chatterModule.Registry.Register(registry.BizType{
		Type: "system_role", DisplayName: "角色", Table: "system_role",
		AuditFields: []registry.AuditField{
			{Column: "name", Field: "Name", Label: "角色名称", Type: "string"},
			{Column: "code", Field: "Code", Label: "角色编码", Type: "string"},
			{Column: "sort", Field: "Sort", Label: "排序", Type: "int"},
			{Column: "status", Field: "Status", Label: "状态", Type: "enum"},
			{Column: "remark", Field: "Remark", Label: "备注", Type: "string"},
			{Column: "data_scope", Field: "DataScope", Label: "数据范围", Type: "enum"},
			{Column: "data_scope_dept_ids", Field: "DataScopeDeptIDs", Label: "数据范围部门", Type: "string"},
			{Column: "menus", Field: "Menus", Label: "菜单权限数", Type: "int"},
			{Column: "field_perm", Field: "FieldPerm", Label: "字段权限", Type: "string"},
		},
	})
	systemModule.SetAuditor(chatterModule.Audit)
	// 自用 ERP 三个聚合接入 chatter 字段审计:保存时字段变更自动生成右侧动态。
	chatterModule.Registry.Register(registry.BizType{
		Type: "myerp_category", DisplayName: "分类", Table: "myerp_category",
		AuditFields: []registry.AuditField{
			{Column: "name", Field: "Name", Label: "名称", Type: "string"},
			{Column: "code", Field: "Code", Label: "编码", Type: "string"},
			{Column: "parent_id", Field: "ParentID", Label: "父分类", Type: "ref"},
			{Column: "sort", Field: "Sort", Label: "排序", Type: "int"},
			{Column: "status", Field: "Status", Label: "状态", Type: "enum"},
			{Column: "inherit_parent_attrs", Field: "InheritParentAttrs", Label: "继承父属性", Type: "bool"},
			{Column: "description", Field: "Description", Label: "说明", Type: "string"},
		},
	})
	chatterModule.Registry.Register(registry.BizType{
		Type: "myerp_attribute", DisplayName: "属性", Table: "myerp_attribute",
		AuditFields: []registry.AuditField{
			{Column: "name", Field: "Name", Label: "名称", Type: "string"},
			{Column: "unit", Field: "Unit", Label: "单位", Type: "string"},
			{Column: "required", Field: "Required", Label: "必填", Type: "bool"},
			{Column: "searchable", Field: "Searchable", Label: "可搜索", Type: "bool"},
			{Column: "show_in_list", Field: "ShowInList", Label: "列表显示", Type: "bool"},
			{Column: "min_value", Field: "MinValue", Label: "最小值", Type: "string"},
			{Column: "max_value", Field: "MaxValue", Label: "最大值", Type: "string"},
			{Column: "regex", Field: "Regex", Label: "正则", Type: "string"},
			{Column: "sort", Field: "Sort", Label: "排序", Type: "int"},
			{Column: "status", Field: "Status", Label: "状态", Type: "enum"},
			{Column: "description", Field: "Description", Label: "说明", Type: "string"},
		},
	})
	chatterModule.Registry.Register(registry.BizType{
		Type: "myerp_product", DisplayName: "产品", Table: "myerp_product",
		AuditFields: []registry.AuditField{
			{Column: "name", Field: "Name", Label: "名称", Type: "string"},
			{Column: "code", Field: "Code", Label: "编码", Type: "string"},
			{Column: "bar_code", Field: "BarCode", Label: "条形码", Type: "string"},
			{Column: "category_id", Field: "CategoryID", Label: "分类", Type: "ref"},
			{Column: "purchase_price", Field: "PurchasePrice", Label: "采购价", Type: "string"},
			{Column: "sale_price", Field: "SalePrice", Label: "销售价", Type: "string"},
			{Column: "stock", Field: "Stock", Label: "库存", Type: "string"},
			{Column: "status", Field: "Status", Label: "状态", Type: "enum"},
			{Column: "owner_user_id", Field: "OwnerUserID", Label: "负责人", Type: "ref"},
			{Column: "description", Field: "Description", Label: "说明", Type: "string"},
		},
	})
	myerpModule.SetAuditor(chatterModule.Audit)
	// 消费者须在事件总线 Start 之前完成订阅。
	if err := chatterModule.RegisterConsumers(bus); err != nil {
		return fmt.Errorf("register chatter consumers: %w", err)
	}
	if err := bus.Start(); err != nil {
		return fmt.Errorf("start eventbus: %w", err)
	}
	// 发件箱投递中继须在事件总线 Start 之后启动。
	chatterModule.StartRelay()
	log.Info("eventbus started", "type", cfg.EventBus.Type)

	// 定时任务调度器
	if err := infraModule.Start(); err != nil {
		return fmt.Errorf("start job scheduler: %w", err)
	}

	// 5. HTTP 引擎与路由
	engine := web.NewEngine(cfg.Server)
	// 限流中间件（全局，按 IP；横切能力 #10）。
	engine.Use(web.RateLimit(rdb.Raw(), 600))
	validator := systemModule.TokenValidator()
	// 免认证接口（登录、租户解析）。
	publicAPI := engine.Group("/admin-api")
	systemModule.RegisterPublic(publicAPI)
	infraModule.RegisterPublic(publicAPI)
	// 需认证接口（认证 → 数据权限 → 操作权限 → 幂等 → 操作日志 → API 访问日志 中间件）。
	adminAPI := engine.Group("/admin-api", security.Auth(validator),
		systemModule.DataPermMiddleware(), systemModule.FieldPermMiddleware(),
		security.RequirePermissionByPath(systemModule.PermissionChecker()),
		web.Idempotent(rdb.Raw()),
		systemModule.OperateLogMiddleware(), infraModule.APIAccessLogMiddleware())
	systemModule.RegisterAuthed(adminAPI)
	infraModule.RegisterAuthed(adminAPI)
	chatterModule.RegisterRoutes(adminAPI)
	myerpModule.RegisterRoutes(adminAPI)
	if cfg.WebSocket.Enable {
		engine.GET(cfg.WebSocket.Path,
			security.Auth(validator),
			websocket.GinHandler(hub),
		)
	}
	// BPM 工作流：方案 A —— 反向代理到独立运行的 BPM Java 服务（yudao-server + Flowable）。
	// 经认证 + API 访问日志中间件：使 BPM 请求带正确的用户信息并记入 infra_api_access_log。
	// BPM 服务自身仍会再校验 token（共享 yudao_go 库的 system_oauth2_access_token）。
	bpmTarget := os.Getenv("BPM_BACKEND")
	if bpmTarget == "" {
		bpmTarget = "http://127.0.0.1:48081"
	}
	bpmProxy, err := web.NewReverseProxyHandler(bpmTarget)
	if err != nil {
		return fmt.Errorf("bpm proxy: %w", err)
	}
	bpmGroup := engine.Group("/admin-api/bpm",
		security.Auth(validator), infraModule.APIAccessLogMiddleware())
	bpmGroup.Any("/*path", bpmProxy)
	log.Info("bpm proxy registered", "target", bpmTarget)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: engine,
	}

	// 7. 启动并等待退出信号
	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// 8. 优雅关闭：先停收新请求，再排空事件总线，最后关闭连接。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	infraModule.Stop() // 停止定时任务调度器
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http server shutdown error", "error", err)
	}
	// 先停投递中继（不再向总线投递），再排空事件总线。
	if err := chatterModule.StopRelay(shutdownCtx); err != nil {
		log.Error("outbox relay stop error", "error", err)
	}
	if err := bus.Stop(shutdownCtx); err != nil {
		log.Error("eventbus stop error", "error", err)
	}
	hub.Close()
	log.Info("server stopped gracefully")
	return nil
}
