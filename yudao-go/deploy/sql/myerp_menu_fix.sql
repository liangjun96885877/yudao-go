-- 修复 myerp 菜单中文乱码(之前 -e 直接传 SQL 时 Windows 终端 UTF-8 → latin1 双重编码)
UPDATE system_menu SET name='自用 ERP' WHERE id=990800;
UPDATE system_menu SET name='分类管理' WHERE id=990810;
UPDATE system_menu SET name='分类查询' WHERE id=990811;
UPDATE system_menu SET name='分类创建' WHERE id=990812;
UPDATE system_menu SET name='分类修改' WHERE id=990813;
UPDATE system_menu SET name='分类删除' WHERE id=990814;
UPDATE system_menu SET name='属性管理' WHERE id=990820;
UPDATE system_menu SET name='属性查询' WHERE id=990821;
UPDATE system_menu SET name='属性创建' WHERE id=990822;
UPDATE system_menu SET name='属性修改' WHERE id=990823;
UPDATE system_menu SET name='属性删除' WHERE id=990824;
UPDATE system_menu SET name='产品管理' WHERE id=990830;
UPDATE system_menu SET name='产品查询' WHERE id=990831;
UPDATE system_menu SET name='产品创建' WHERE id=990832;
UPDATE system_menu SET name='产品修改' WHERE id=990833;
UPDATE system_menu SET name='产品删除' WHERE id=990834;
UPDATE system_menu SET name='产品导出' WHERE id=990835;
