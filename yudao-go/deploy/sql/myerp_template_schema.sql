-- 自用 ERP 产品模板/变体(借鉴 Odoo product.template + product.product 双层模型,精简版)
--
-- 解决「同一产品概念有多个 SKU」(玫瑰/薰衣草/茶树手工皂、iPhone 红 256G/蓝 128G):
--   - 模板(SPU): 共享字段集中(名称/分类/单位/基础售价)
--   - 变体(SKU): 现有 myerp_product 当变体表;加 template_id 可空挂模板
--   - 区分属性: 复用现有 EAV(myerp_attribute 加 is_variant 标记)
--   - 价格加价: 复用 myerp_attribute_value_option(加 price_extra 列)
--   - 共享字段灰显 / 改请回模板,变体只存差异
--
-- 与现状零冲突: template_id 可空,挂了 = 变体、没挂 = 独立 SKU(现状)。
-- 不引入 Odoo 的 _inherits 透明继承,而是在 service 层显式 resolve。
--
-- 导入(PowerShell UTF-8 stdin,菜单中文不乱码):
--   Get-Content deploy/sql/myerp_template_schema.sql -Raw -Encoding UTF8 | `
--     docker exec -i yudao-go-mysql mysql -uroot -p123456 --default-character-set=utf8mb4 yudao_go

-- ===== 1. 产品模板(SPU)=====
CREATE TABLE IF NOT EXISTS `myerp_product_template` (
  `id`           BIGINT NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(128) NOT NULL COMMENT '模板名(同族变体共享)',
  `code`         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '模板代码(可选,租户唯一)',
  `category_id`  BIGINT       NOT NULL,
  `base_uom_id`  BIGINT       NULL COMMENT '基本单位(同族变体强制同单位)',
  `base_price`   DECIMAL(20,6) NOT NULL DEFAULT 0 COMMENT '模板基础售价(变体最终 = base + Σ option.price_extra)',
  `description`  VARCHAR(1024) NULL DEFAULT '',
  `status`       TINYINT NOT NULL DEFAULT 0 COMMENT '0=启用 1=停用',
  `creator`      VARCHAR(64) NULL DEFAULT '',
  `create_time`  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`      VARCHAR(64) NULL DEFAULT '',
  `update_time`  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted`      TINYINT NOT NULL DEFAULT 0,
  `tenant_id`    BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_code` (`tenant_id`, `code`, `deleted`),
  KEY `idx_category` (`category_id`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 产品模板(SPU)';

-- ===== 2. 模板用了哪些区分属性 =====
-- 模板可用的具体选项 = attribute_value_option 当前启用的全集(简化,不再做子集筛选)
CREATE TABLE IF NOT EXISTS `myerp_template_attribute_line` (
  `id`           BIGINT NOT NULL AUTO_INCREMENT,
  `template_id`  BIGINT NOT NULL,
  `attribute_id` BIGINT NOT NULL COMMENT '必须是 is_variant=1 的属性',
  `sort`         INT    NOT NULL DEFAULT 0,
  `create_time`  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `tenant_id`    BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tpl_attr` (`tenant_id`, `template_id`, `attribute_id`),
  KEY `idx_template` (`template_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 模板的区分属性配置';

-- ===== 3. 属性表加 is_variant =====
ALTER TABLE `myerp_attribute`
  ADD COLUMN `is_variant` TINYINT NOT NULL DEFAULT 0
    COMMENT '1=区分属性(可产生变体)0=描述属性' AFTER `show_in_list`;

-- ===== 4. 枚举选项加 price_extra(加价)=====
ALTER TABLE `myerp_attribute_value_option`
  ADD COLUMN `price_extra` DECIMAL(20,6) NOT NULL DEFAULT 0
    COMMENT '该选项的加价(SKU 售价 = template.base_price + Σ 所选选项 price_extra)' AFTER `sort`;

-- ===== 5. 产品(SKU)加 template_id(可空)=====
ALTER TABLE `myerp_product`
  ADD COLUMN `template_id` BIGINT NULL COMMENT '所属模板;null=独立 SKU' AFTER `category_id`,
  ADD KEY `idx_template` (`template_id`);

-- ===== 6. 模板管理菜单 + 权限点(ID 990860 段)=====
INSERT IGNORE INTO `system_menu`
  (`id`, `name`, `permission`, `type`, `sort`, `parent_id`, `path`, `icon`, `component`, `component_name`, `status`, `creator`, `create_time`, `updater`, `update_time`, `deleted`)
VALUES
  (990860, '模板管理', '',                      2, 6, 990800, 'template', '', 'myerp/template/index', 'MyErpTemplate', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990861, '模板查询', 'myerp:template:query',  3, 1, 990860, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990862, '模板创建', 'myerp:template:create', 3, 2, 990860, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990863, '模板修改', 'myerp:template:update', 3, 3, 990860, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990864, '模板删除', 'myerp:template:delete', 3, 4, 990860, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990865, '生成变体', 'myerp:template:gen',    3, 5, 990860, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0');

INSERT IGNORE INTO `system_role_menu` (role_id, menu_id, creator, create_time, updater, update_time, deleted, tenant_id)
SELECT 1, id, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0', 1
FROM `system_menu` WHERE id BETWEEN 990860 AND 990865;
