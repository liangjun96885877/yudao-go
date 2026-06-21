-- 自用 ERP 多单位换算(企业级)—— 单位字典 + 产品多单位换算
--
-- 模型:
--   myerp_uom            全局单位字典(颗/斤/箱/千克…)
--   myerp_product.base_uom_id   产品基本单位(库存单位,最小不可分)
--   myerp_product_uom    产品的辅助单位 + 换算系数(产品级,支持浮动换算)
--
-- 换算公式:基本单位数量 = 辅助单位数量 × factor
--   螺丝 base=颗;辅助单位 斤 factor=50 → 2 斤 = 100 颗
--
-- 设计取舍:
--   - category 仅分组展示,不强制同类才能换算(螺丝斤↔颗本就跨类)
--   - 换算率存产品级(product_uom.factor),支持浮动换算(因品而异)
--   - 库存/价格仍以基本单位为单一真相源,展示时换算
--
-- 导入: docker exec -i yudao-go-mysql mysql -uroot -p123456 yudao_go < myerp_uom_schema.sql

-- ===== 1. 单位字典 =====
CREATE TABLE IF NOT EXISTS `myerp_uom` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(32) NOT NULL COMMENT '单位名称(颗/斤/箱/千克)',
  `code` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '单位编码(租户内唯一,pcs/jin/box/kg)',
  `category` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '类别(count数量/weight重量/length长度,仅分组展示)',
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
  UNIQUE KEY `uk_tenant_code` (`tenant_id`, `code`, `deleted`),
  KEY `idx_status` (`tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 单位字典';

-- ===== 2. 产品基本单位字段 =====
-- base_uom_id:产品的库存/基本单位,所有库存与换算以此为基准
ALTER TABLE `myerp_product`
  ADD COLUMN `base_uom_id` BIGINT NULL COMMENT '基本单位(库存单位)id → myerp_uom' AFTER `category_id`;

-- ===== 3. 产品多单位换算(子表) =====
CREATE TABLE IF NOT EXISTS `myerp_product_uom` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `product_id` BIGINT NOT NULL,
  `uom_id` BIGINT NOT NULL COMMENT '辅助单位 id → myerp_uom',
  `factor` DECIMAL(20,6) NOT NULL DEFAULT 1 COMMENT '换算系数:1 辅助单位 = factor 基本单位(螺丝斤=50)',
  `is_purchase` TINYINT NOT NULL DEFAULT 0 COMMENT '默认采购单位',
  `is_sale` TINYINT NOT NULL DEFAULT 0 COMMENT '默认销售单位',
  `sort` INT NOT NULL DEFAULT 0,
  `creator` VARCHAR(64) NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater` VARCHAR(64) NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  -- 一个产品对一个辅助单位只配一行
  UNIQUE KEY `uk_product_uom` (`tenant_id`, `product_id`, `uom_id`, `deleted`),
  KEY `idx_product` (`product_id`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 产品多单位换算';

-- ===== 4. 单位管理菜单 + 权限点(ID 990840 段) =====
INSERT IGNORE INTO `system_menu`
  (`id`, `name`, `permission`, `type`, `sort`, `parent_id`, `path`, `icon`, `component`, `component_name`, `status`, `creator`, `create_time`, `updater`, `update_time`, `deleted`)
VALUES
  (990840, '单位管理', '',                 2, 4, 990800, 'uom', '', 'myerp/uom/index', 'MyErpUom', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990841, '单位查询', 'myerp:uom:query',  3, 1, 990840, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990842, '单位创建', 'myerp:uom:create', 3, 2, 990840, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990843, '单位修改', 'myerp:uom:update', 3, 3, 990840, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990844, '单位删除', 'myerp:uom:delete', 3, 4, 990840, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0');

INSERT IGNORE INTO `system_role_menu` (role_id, menu_id, creator, create_time, updater, update_time, deleted, tenant_id)
SELECT 1, id, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0', 1
FROM `system_menu` WHERE id BETWEEN 990840 AND 990844;
