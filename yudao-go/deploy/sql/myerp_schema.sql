-- "自用 ERP 系统" EAV（Entity-Attribute-Value）模型 —— 5 张表
-- 分类(树) + 属性定义 + 属性枚举值 + 产品主表 + 产品属性值(EAV 核心)
--
-- 参考一个 go-zero 微服务原型的 EAV 表结构(已通过原项目 10 项安全审查),
-- 结构基本不改:
--   - 所有写表都带 tenant_id 索引(多租户行级隔离)
--   - 唯一约束:DB 层兜底(应用 + DB 双校验防并发 TOCTOU)
--   - VARCHAR 长度上限(防大请求体 DoS)
--   - 属性 value <= 1024 字符
--   - 软删除 deleted TINYINT(0=未删/1=已删)对齐 GORM soft_delete flag
--   - sort/status/creator/updater 审计列统一
--
-- 导入: docker exec -i yudao-go-mysql mysql -uroot -p123456 yudao_go < myerp_schema.sql

-- ===== 1. 分类(树形) =====
CREATE TABLE IF NOT EXISTS `myerp_category` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL COMMENT '分类名称',
  `parent_id` BIGINT NOT NULL DEFAULT 0 COMMENT '父分类(0=根)',
  `code` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '分类编码(租户内唯一)',
  `sort` INT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0=启用 1=停用',
  `inherit_parent_attrs` BIT(1) NOT NULL DEFAULT b'1' COMMENT '是否继承父分类属性',
  `description` VARCHAR(255) NULL DEFAULT '',
  `creator` VARCHAR(64) NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater` VARCHAR(64) NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_code` (`tenant_id`, `code`, `deleted`),
  KEY `idx_parent` (`parent_id`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 分类(树形)';

-- ===== 2. 属性定义 =====
CREATE TABLE IF NOT EXISTS `myerp_attribute` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `category_id` BIGINT NOT NULL COMMENT '所属分类',
  `code` VARCHAR(32) NOT NULL COMMENT '属性编码(如 brand、color)',
  `name` VARCHAR(64) NOT NULL COMMENT '属性显示名',
  `input_type` VARCHAR(16) NOT NULL COMMENT 'text|number|select|multi_select|bool|date|datetime|url|color',
  `unit` VARCHAR(16) NULL DEFAULT '' COMMENT '单位(如 mm、kg、英寸)',
  `required` BIT(1) NOT NULL DEFAULT b'0' COMMENT '是否必填',
  `searchable` BIT(1) NOT NULL DEFAULT b'0' COMMENT '是否参与列表筛选',
  `show_in_list` BIT(1) NOT NULL DEFAULT b'1' COMMENT '列表页是否显示该列',
  `min_value` DECIMAL(20,6) NULL COMMENT 'number 最小值',
  `max_value` DECIMAL(20,6) NULL COMMENT 'number 最大值',
  `min_length` INT NULL COMMENT 'text 最小长度',
  `max_length` INT NOT NULL DEFAULT 1024 COMMENT 'text 最大长度(硬上限 1024)',
  `regex` VARCHAR(255) NULL COMMENT 'text 校验正则',
  `default_value` VARCHAR(255) NULL DEFAULT '',
  `sort` INT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0=启用 1=停用',
  `description` VARCHAR(255) NULL DEFAULT '',
  `creator` VARCHAR(64) NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater` VARCHAR(64) NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_cat_code` (`tenant_id`, `category_id`, `code`, `deleted`),
  KEY `idx_category` (`category_id`, `tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 属性定义(EAV 字典)';

-- ===== 3. 属性枚举值(仅 select/multi_select 用) =====
CREATE TABLE IF NOT EXISTS `myerp_attribute_value_option` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `attribute_id` BIGINT NOT NULL,
  `value` VARCHAR(255) NOT NULL COMMENT '选项值',
  `sort` INT NOT NULL DEFAULT 0,
  `creator` VARCHAR(64) NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater` VARCHAR(64) NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_attr_value` (`tenant_id`, `attribute_id`, `value`, `deleted`),
  KEY `idx_attr` (`attribute_id`, `tenant_id`, `sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 属性枚举选项';

-- ===== 4. 产品主表 =====
CREATE TABLE IF NOT EXISTS `myerp_product` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `category_id` BIGINT NOT NULL COMMENT '所属分类',
  `code` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '产品编号',
  `name` VARCHAR(128) NOT NULL,
  `bar_code` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '条形码',
  `pic_url` VARCHAR(512) NOT NULL DEFAULT '',
  `description` VARCHAR(1024) NOT NULL DEFAULT '',
  `purchase_price` DECIMAL(20,2) NOT NULL DEFAULT 0,
  `sale_price` DECIMAL(20,2) NOT NULL DEFAULT 0,
  `stock` DECIMAL(20,3) NOT NULL DEFAULT 0 COMMENT '当前库存',
  `status` TINYINT NOT NULL DEFAULT 0,
  `owner_user_id` BIGINT NULL COMMENT '负责人(数据权限基础)',
  `creator` VARCHAR(64) NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater` VARCHAR(64) NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_code` (`tenant_id`, `code`, `deleted`),
  KEY `idx_category` (`category_id`, `tenant_id`, `deleted`),
  KEY `idx_bar_code` (`bar_code`, `tenant_id`),
  KEY `idx_owner` (`owner_user_id`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 产品主表';

-- ===== 5. 产品属性值(EAV 核心) =====
CREATE TABLE IF NOT EXISTS `myerp_product_attr_value` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `product_id` BIGINT NOT NULL,
  `attribute_id` BIGINT NOT NULL,
  `attribute_code` VARCHAR(32) NOT NULL COMMENT '冗余 attribute.code 便于不 join 取值',
  `value` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '主存储(统一 varchar 1024)',
  `value_decimal` DECIMAL(20,6) NULL COMMENT 'number 类型冗余,范围筛选用',
  `value_date` DATETIME NULL COMMENT 'date/datetime 类型冗余',
  `value_bool` BIT(1) NULL COMMENT 'bool 类型冗余',
  `creator` VARCHAR(64) NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater` VARCHAR(64) NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_product_attr` (`tenant_id`, `product_id`, `attribute_id`, `deleted`),
  KEY `idx_attr_value` (`tenant_id`, `attribute_id`, `value`(64)),
  KEY `idx_attr_decimal` (`tenant_id`, `attribute_id`, `value_decimal`),
  KEY `idx_attr_date` (`tenant_id`, `attribute_id`, `value_date`),
  KEY `idx_product` (`product_id`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 产品-属性值(EAV 核心)';

-- ===== 菜单 + 权限点初始化(ID 段 990800-990899,避免与现有冲突) =====
INSERT IGNORE INTO `system_menu`
  (`id`, `name`, `permission`, `type`, `sort`, `parent_id`, `path`, `icon`, `component`, `component_name`, `status`, `creator`, `create_time`, `updater`, `update_time`, `deleted`)
VALUES
  (990800, '自用 ERP',      '',                       1, 80, 0,      '/myerp',           'ep:goods',     '',                                'MyErp',           0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  -- 分类
  (990810, '分类管理',      '',                       2, 1,  990800, 'category',         '',             'myerp/category/index',            'MyErpCategory',   0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990811, '分类查询',      'myerp:category:query',   3, 1,  990810, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990812, '分类创建',      'myerp:category:create',  3, 2,  990810, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990813, '分类修改',      'myerp:category:update',  3, 3,  990810, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990814, '分类删除',      'myerp:category:delete',  3, 4,  990810, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  -- 属性
  (990820, '属性管理',      '',                       2, 2,  990800, 'attribute',        '',             'myerp/attribute/index',           'MyErpAttribute',  0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990821, '属性查询',      'myerp:attribute:query',  3, 1,  990820, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990822, '属性创建',      'myerp:attribute:create', 3, 2,  990820, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990823, '属性修改',      'myerp:attribute:update', 3, 3,  990820, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990824, '属性删除',      'myerp:attribute:delete', 3, 4,  990820, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  -- 产品
  (990830, '产品管理',      '',                       2, 3,  990800, 'product',          '',             'myerp/product/index',             'MyErpProduct',    0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990831, '产品查询',      'myerp:product:query',    3, 1,  990830, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990832, '产品创建',      'myerp:product:create',   3, 2,  990830, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990833, '产品修改',      'myerp:product:update',   3, 3,  990830, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990834, '产品删除',      'myerp:product:delete',   3, 4,  990830, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990835, '产品导出',      'myerp:product:export',   3, 5,  990830, '',                 '',             '',                                '',                0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0');

-- super_admin 角色关联
INSERT IGNORE INTO `system_role_menu` (role_id, menu_id, creator, create_time, updater, update_time, deleted, tenant_id)
SELECT 1, id, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0', 1
FROM `system_menu` WHERE id BETWEEN 990800 AND 990899;
