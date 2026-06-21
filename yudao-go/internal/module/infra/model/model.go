// Package model 是 infra（基础设施）模块的持久化模型。
package model

import (
	"time"

	"yudao-go/internal/framework/orm"
)

// Base 是 infra 表公共字段。不映射 deleted（原版 bit(1)），由仓储显式过滤/软删。
type Base struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Creator    string    `gorm:"column:creator" json:"creator"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	Updater    string    `gorm:"column:updater" json:"updater"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

// Config 对应 infra_config。
type Config struct {
	Base
	Category  string  `gorm:"column:category" json:"category"`
	Type      int8    `gorm:"column:type" json:"type"`
	Name      string  `gorm:"column:name" json:"name"`
	ConfigKey string  `gorm:"column:config_key" json:"key"`
	Value     string  `gorm:"column:value" json:"value"`
	Visible   orm.Bit `gorm:"column:visible" json:"visible"`
	Remark    string  `gorm:"column:remark" json:"remark"`
}

func (Config) TableName() string { return "infra_config" }

// Job 对应 infra_job（定时任务）。
type Job struct {
	Base
	Name           string `gorm:"column:name" json:"name"`
	Status         int8   `gorm:"column:status" json:"status"`
	HandlerName    string `gorm:"column:handler_name" json:"handlerName"`
	HandlerParam   string `gorm:"column:handler_param" json:"handlerParam"`
	CronExpression string `gorm:"column:cron_expression" json:"cronExpression"`
	RetryCount     int    `gorm:"column:retry_count" json:"retryCount"`
	RetryInterval  int    `gorm:"column:retry_interval" json:"retryInterval"`
	MonitorTimeout int    `gorm:"column:monitor_timeout" json:"monitorTimeout"`
}

func (Job) TableName() string { return "infra_job" }

// DataSourceConfig 对应 infra_data_source_config（数据源配置）。
type DataSourceConfig struct {
	Base
	Name     string `gorm:"column:name" json:"name"`
	URL      string `gorm:"column:url" json:"url"`
	Username string `gorm:"column:username" json:"username"`
	Password string `gorm:"column:password" json:"password"`
}

func (DataSourceConfig) TableName() string { return "infra_data_source_config" }

// JobLog 对应 infra_job_log（定时任务执行日志）。
type JobLog struct {
	Base
	JobID        int64     `gorm:"column:job_id" json:"jobId"`
	HandlerName  string    `gorm:"column:handler_name" json:"handlerName"`
	HandlerParam string    `gorm:"column:handler_param" json:"handlerParam"`
	ExecuteIndex int       `gorm:"column:execute_index" json:"executeIndex"`
	BeginTime    time.Time `gorm:"column:begin_time" json:"beginTime"`
	EndTime      time.Time `gorm:"column:end_time" json:"endTime"`
	Duration     int       `gorm:"column:duration" json:"duration"`
	Status       int8      `gorm:"column:status" json:"status"` // 1 运行中 2 成功 3 失败
	Result       string    `gorm:"column:result" json:"result"`
}

func (JobLog) TableName() string { return "infra_job_log" }

// File 对应 infra_file（文件记录）。
type File struct {
	Base
	ConfigID int64  `gorm:"column:config_id" json:"configId"`
	Name     string `gorm:"column:name" json:"name"`
	Path     string `gorm:"column:path" json:"path"`
	URL      string `gorm:"column:url" json:"url"`
	Type     string `gorm:"column:type" json:"type"`
	Size     int    `gorm:"column:size" json:"size"`
}

func (File) TableName() string { return "infra_file" }

// FileContent 对应 infra_file_content（文件内容，DB 存储）。
type FileContent struct {
	Base
	ConfigID int64  `gorm:"column:config_id" json:"-"`
	Path     string `gorm:"column:path" json:"-"`
	Content  []byte `gorm:"column:content" json:"-"`
}

func (FileContent) TableName() string { return "infra_file_content" }

// FileConfig 对应 infra_file_config（文件存储配置）。
type FileConfig struct {
	Base
	Name    string  `gorm:"column:name" json:"name"`
	Storage int8    `gorm:"column:storage" json:"storage"`
	Remark  string  `gorm:"column:remark" json:"remark"`
	Master  orm.Bit `gorm:"column:master" json:"master"`
	Config  string  `gorm:"column:config" json:"-"`
}

func (FileConfig) TableName() string { return "infra_file_config" }

// ApiAccessLog 对应 infra_api_access_log（API 访问日志）。
type ApiAccessLog struct {
	Base
	TenantID        int64     `gorm:"column:tenant_id" json:"tenantId"`
	TraceID         string    `gorm:"column:trace_id" json:"traceId"`
	UserID          int64     `gorm:"column:user_id" json:"userId"`
	UserType        int8      `gorm:"column:user_type" json:"userType"`
	ApplicationName string    `gorm:"column:application_name" json:"applicationName"`
	RequestMethod   string    `gorm:"column:request_method" json:"requestMethod"`
	RequestURL      string    `gorm:"column:request_url" json:"requestUrl"`
	RequestParams   string    `gorm:"column:request_params" json:"requestParams"`
	ResponseBody    string    `gorm:"column:response_body" json:"responseBody"`
	UserIP          string    `gorm:"column:user_ip" json:"userIp"`
	UserAgent       string    `gorm:"column:user_agent" json:"userAgent"`
	OperateModule   string    `gorm:"column:operate_module" json:"operateModule"`
	OperateName     string    `gorm:"column:operate_name" json:"operateName"`
	OperateType     int8      `gorm:"column:operate_type" json:"operateType"`
	BeginTime       time.Time `gorm:"column:begin_time" json:"beginTime"`
	EndTime         time.Time `gorm:"column:end_time" json:"endTime"`
	Duration        int       `gorm:"column:duration" json:"duration"`
	ResultCode      int       `gorm:"column:result_code" json:"resultCode"`
	ResultMsg       string    `gorm:"column:result_msg" json:"resultMsg"`
}

func (ApiAccessLog) TableName() string { return "infra_api_access_log" }

// ApiErrorLog 对应 infra_api_error_log（API 错误日志）。
type ApiErrorLog struct {
	Base
	TenantID                      int64      `gorm:"column:tenant_id" json:"tenantId"`
	TraceID                       string     `gorm:"column:trace_id" json:"traceId"`
	UserID                        int64      `gorm:"column:user_id" json:"userId"`
	UserType                      int8       `gorm:"column:user_type" json:"userType"`
	ApplicationName               string     `gorm:"column:application_name" json:"applicationName"`
	RequestMethod                 string     `gorm:"column:request_method" json:"requestMethod"`
	RequestURL                    string     `gorm:"column:request_url" json:"requestUrl"`
	RequestParams                 string     `gorm:"column:request_params" json:"requestParams"`
	UserIP                        string     `gorm:"column:user_ip" json:"userIp"`
	UserAgent                     string     `gorm:"column:user_agent" json:"userAgent"`
	ExceptionTime                 time.Time  `gorm:"column:exception_time" json:"exceptionTime"`
	ExceptionName                 string     `gorm:"column:exception_name" json:"exceptionName"`
	ExceptionMessage              string     `gorm:"column:exception_message" json:"exceptionMessage"`
	ExceptionRootCauseMessage     string     `gorm:"column:exception_root_cause_message" json:"exceptionRootCauseMessage"`
	ExceptionStackTrace           string     `gorm:"column:exception_stack_trace" json:"exceptionStackTrace"`
	ExceptionClassName            string     `gorm:"column:exception_class_name" json:"exceptionClassName"`
	ExceptionFileName             string     `gorm:"column:exception_file_name" json:"exceptionFileName"`
	ExceptionMethodName           string     `gorm:"column:exception_method_name" json:"exceptionMethodName"`
	ExceptionLineNumber           int        `gorm:"column:exception_line_number" json:"exceptionLineNumber"`
	ProcessStatus                 int8       `gorm:"column:process_status" json:"processStatus"`
	ProcessTime                   *time.Time `gorm:"column:process_time" json:"processTime"`
	ProcessUserID                 int64      `gorm:"column:process_user_id" json:"processUserId"`
}

func (ApiErrorLog) TableName() string { return "infra_api_error_log" }

// CodegenTable 对应 infra_codegen_table（代码生成 - 表定义）。
type CodegenTable struct {
	Base
	DataSourceConfigID int64   `gorm:"column:data_source_config_id" json:"dataSourceConfigId"`
	Scene              int8    `gorm:"column:scene" json:"scene"`
	Name               string  `gorm:"column:table_name" json:"tableName"`
	TableComment       string  `gorm:"column:table_comment" json:"tableComment"`
	Remark             string  `gorm:"column:remark" json:"remark"`
	ModuleName         string  `gorm:"column:module_name" json:"moduleName"`
	BusinessName       string  `gorm:"column:business_name" json:"businessName"`
	ClassName          string  `gorm:"column:class_name" json:"className"`
	ClassComment       string  `gorm:"column:class_comment" json:"classComment"`
	Author             string  `gorm:"column:author" json:"author"`
	TemplateType       int8    `gorm:"column:template_type" json:"templateType"`
	FrontType          int8    `gorm:"column:front_type" json:"frontType"`
	ParentMenuID       int64   `gorm:"column:parent_menu_id" json:"parentMenuId"`
	MasterTableID      int64   `gorm:"column:master_table_id" json:"masterTableId"`
	SubJoinColumnID    int64   `gorm:"column:sub_join_column_id" json:"subJoinColumnId"`
	SubJoinMany        orm.Bit `gorm:"column:sub_join_many" json:"subJoinMany"`
	TreeParentColumnID int64   `gorm:"column:tree_parent_column_id" json:"treeParentColumnId"`
	TreeNameColumnID   int64   `gorm:"column:tree_name_column_id" json:"treeNameColumnId"`
}

func (CodegenTable) TableName() string { return "infra_codegen_table" }

// CodegenColumn 对应 infra_codegen_column（代码生成 - 字段定义）。
type CodegenColumn struct {
	Base
	TableID                int64   `gorm:"column:table_id" json:"tableId"`
	ColumnName             string  `gorm:"column:column_name" json:"columnName"`
	DataType               string  `gorm:"column:data_type" json:"dataType"`
	ColumnComment          string  `gorm:"column:column_comment" json:"columnComment"`
	Nullable               orm.Bit `gorm:"column:nullable" json:"nullable"`
	PrimaryKey             orm.Bit `gorm:"column:primary_key" json:"primaryKey"`
	OrdinalPosition        int     `gorm:"column:ordinal_position" json:"ordinalPosition"`
	JavaType               string  `gorm:"column:java_type" json:"javaType"`
	JavaField              string  `gorm:"column:java_field" json:"javaField"`
	DictType               string  `gorm:"column:dict_type" json:"dictType"`
	Example                string  `gorm:"column:example" json:"example"`
	CreateOperation        orm.Bit `gorm:"column:create_operation" json:"createOperation"`
	UpdateOperation        orm.Bit `gorm:"column:update_operation" json:"updateOperation"`
	ListOperation          orm.Bit `gorm:"column:list_operation" json:"listOperation"`
	ListOperationCondition string  `gorm:"column:list_operation_condition" json:"listOperationCondition"`
	ListOperationResult    orm.Bit `gorm:"column:list_operation_result" json:"listOperationResult"`
	HtmlType               string  `gorm:"column:html_type" json:"htmlType"`
}

func (CodegenColumn) TableName() string { return "infra_codegen_column" }
