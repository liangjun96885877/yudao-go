-- 自用 ERP 双计量 + 批次(企业级 Catch Weight)
--
-- 解决「同一产品因批次不同换算率不同」(手工件:这批 10 克/个、下批 10.2 克/个):
--   - 产品加「换算方式」开关:固定(原行为) / 浮动双计量
--   - 浮动产品:主计量 + 辅计量两列独立结存,不靠系数互推
--   - 批次(myerp_product_batch):每批存实测换算率 actual_factor(因批而异落点)
--   - 出入库流水(myerp_stock_move):账本,库存 = 流水之和;批次/产品结存是投影
--   - 名义换算率 nominal_factor + 容差 tolerance_pct:录入默认值 + 偏差校验,不当记账依据
--
-- 共存:存量产品默认 uom_mode=0(固定),行为零变化;批次/流水表不启用即为空。
--
-- 导入(PowerShell,务必带 --default-character-set=utf8mb4,否则菜单中文双重编码乱码):
--   Get-Content deploy/sql/myerp_batch_schema.sql -Raw -Encoding UTF8 | `
--     docker exec -i yudao-go-mysql mysql -uroot -p123456 --default-character-set=utf8mb4 yudao_go
-- 若已乱码,执行 deploy/sql/myerp_batch_menu_fix.sql 修复菜单名。

-- ===== 1. 产品增列(双计量配置) =====
ALTER TABLE `myerp_product`
  ADD COLUMN `uom_mode`       TINYINT       NOT NULL DEFAULT 0 COMMENT '换算方式 0=固定 1=浮动双计量' AFTER `base_uom_id`,
  ADD COLUMN `aux_uom_id`     BIGINT        NULL     COMMENT '辅计量单位(异重单位,如克)→ myerp_uom' AFTER `uom_mode`,
  ADD COLUMN `nominal_factor` DECIMAL(20,6) NULL     COMMENT '名义换算率 1主≈N辅(默认值+校验基准)' AFTER `aux_uom_id`,
  ADD COLUMN `tolerance_pct`  DECIMAL(8,4)  NOT NULL DEFAULT 0 COMMENT '允许偏差% 0=不校验' AFTER `nominal_factor`,
  ADD COLUMN `stock_aux`      DECIMAL(20,6) NOT NULL DEFAULT 0 COMMENT '辅计量库存合计(浮动模式)' AFTER `stock`;

-- ===== 2. 批次主数据 =====
CREATE TABLE IF NOT EXISTS `myerp_product_batch` (
  `id`            BIGINT NOT NULL AUTO_INCREMENT,
  `product_id`    BIGINT NOT NULL,
  `batch_no`      VARCHAR(64) NOT NULL COMMENT '批次号 20260529-001',
  `actual_factor` DECIMAL(20,6) NULL COMMENT '该批实测换算率(因批而异:10.0 / 10.2)',
  `stock_base`    DECIMAL(20,6) NOT NULL DEFAULT 0 COMMENT '该批主计量结存',
  `stock_aux`     DECIMAL(20,6) NOT NULL DEFAULT 0 COMMENT '该批辅计量结存',
  `produce_date`  DATE NULL,
  `expire_date`   DATE NULL,
  `status`        TINYINT NOT NULL DEFAULT 0 COMMENT '0=正常 1=冻结',
  `remark`        VARCHAR(255) NULL DEFAULT '',
  `creator`       VARCHAR(64) NULL DEFAULT '',
  `create_time`   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64) NULL DEFAULT '',
  `update_time`   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted`       TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  `tenant_id`     BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_batch` (`tenant_id`, `product_id`, `batch_no`, `deleted`),
  KEY `idx_product` (`product_id`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 产品批次';

-- ===== 3. 出入库流水(账本,不可改不可删) =====
CREATE TABLE IF NOT EXISTS `myerp_stock_move` (
  `id`               BIGINT NOT NULL AUTO_INCREMENT,
  `product_id`       BIGINT NOT NULL,
  `batch_id`         BIGINT NULL COMMENT '非批次产品为空',
  `move_type`        TINYINT NOT NULL COMMENT '1=入库 2=出库 3=盘点调整',
  `qty_base`         DECIMAL(20,6) NOT NULL COMMENT '主计量变动(入正出负)',
  `qty_aux`          DECIMAL(20,6) NOT NULL DEFAULT 0 COMMENT '辅计量变动(双计量时必填)',
  `effective_factor` DECIMAL(20,6) NULL COMMENT '本次实际换算率=|qty_aux/qty_base|(留痕)',
  `biz_type`         VARCHAR(32) NULL DEFAULT '' COMMENT '来源 purchase_in/sale_out/adjust',
  `biz_id`           BIGINT NULL,
  `remark`           VARCHAR(255) NULL DEFAULT '',
  `creator`          VARCHAR(64) NULL DEFAULT '',
  `create_time`      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `tenant_id`        BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_feed` (`tenant_id`, `product_id`, `id`),
  KEY `idx_batch` (`batch_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='myerp 出入库流水(账本)';

-- ===== 4. 批次管理菜单 + 权限点(ID 990850 段) =====
INSERT IGNORE INTO `system_menu`
  (`id`, `name`, `permission`, `type`, `sort`, `parent_id`, `path`, `icon`, `component`, `component_name`, `status`, `creator`, `create_time`, `updater`, `update_time`, `deleted`)
VALUES
  (990850, '批次管理', '',                   2, 5, 990800, 'batch', '', 'myerp/batch/index', 'MyErpBatch', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990851, '批次查询', 'myerp:batch:query',  3, 1, 990850, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990852, '批次创建', 'myerp:batch:create', 3, 2, 990850, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990853, '批次修改', 'myerp:batch:update', 3, 3, 990850, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990854, '批次删除', 'myerp:batch:delete', 3, 4, 990850, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0'),
  (990855, '出入库',   'myerp:stock:move',   3, 5, 990850, '', '', '', '', 0, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0');

INSERT IGNORE INTO `system_role_menu` (role_id, menu_id, creator, create_time, updater, update_time, deleted, tenant_id)
SELECT 1, id, 'myerp-init', NOW(), 'myerp-init', NOW(), b'0', 1
FROM `system_menu` WHERE id BETWEEN 990850 AND 990855;
