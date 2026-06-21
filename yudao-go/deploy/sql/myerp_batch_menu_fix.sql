-- 修复 myerp_batch_schema.sql 导入时未带 --default-character-set=utf8mb4 导致的菜单中文双重编码。
-- 用法(PowerShell,UTF-8 stdin 管道):
--   Get-Content deploy/sql/myerp_batch_menu_fix.sql -Raw -Encoding UTF8 | `
--     docker exec -i yudao-go-mysql mysql -uroot -p123456 --default-character-set=utf8mb4 yudao_go
UPDATE `system_menu` SET `name`='批次管理' WHERE id=990850;
UPDATE `system_menu` SET `name`='批次查询' WHERE id=990851;
UPDATE `system_menu` SET `name`='批次创建' WHERE id=990852;
UPDATE `system_menu` SET `name`='批次修改' WHERE id=990853;
UPDATE `system_menu` SET `name`='批次删除' WHERE id=990854;
UPDATE `system_menu` SET `name`='出入库'   WHERE id=990855;
