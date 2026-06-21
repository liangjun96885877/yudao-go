-- 自用 ERP 双计量 batch-less 模式(随机重量 / Random Weight Catch Weight)
--
-- 解决「每笔重量都不同 + 不关心批次」的纯异重场景(生鲜/钢板/散装/废料)。
-- 在产品级加 batch_tracked 开关:
--   - 1(默认,沿用现状):浮动产品出入库必须挂批次,实测率/结存按批管理。
--   - 0(新增 batch-less):浮动产品出入库直接落产品级,batch_id 可空,
--     每笔流水自带主/辅双数量+实际换算率,容差按笔校验,库存按产品级双列累加。
--
-- 库存仍以「主+辅双列独立累加」为唯一真相源,改不会乱。
--
-- 导入(无中文,可裸 -i 也行;为统一仍走 UTF-8 stdin):
--   Get-Content deploy/sql/myerp_batchless_schema.sql -Raw -Encoding UTF8 | `
--     docker exec -i yudao-go-mysql mysql -uroot -p123456 --default-character-set=utf8mb4 yudao_go

ALTER TABLE `myerp_product`
  ADD COLUMN `batch_tracked` TINYINT NOT NULL DEFAULT 1
    COMMENT '是否按批次管理库存(1=按批次,0=batch-less 随机重量;仅浮动双计量产品有效)'
  AFTER `tolerance_pct`;
