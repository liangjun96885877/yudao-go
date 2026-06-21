package service

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"yudao-go/internal/framework/contextx"
	"yudao-go/internal/framework/orm"
)

// DataPermService 解析用户的有效数据权限范围（横切能力 #6）。
type DataPermService struct {
	tx *orm.TxManager
}

func NewDataPermService(tx *orm.TxManager) *DataPermService {
	return &DataPermService{tx: tx}
}

// Resolve 计算 userID 的有效数据权限。多角色取并集，任一为「全部」即 All。
func (s *DataPermService) Resolve(ctx context.Context, userID int64) (*contextx.DataPerm, error) {
	// 解析过程本身不受数据权限影响，避免自我递归过滤。
	ctx = contextx.WithIgnoreDataPerm(ctx)
	db := s.tx.DB(ctx)

	var deptID int64
	db.Raw("SELECT dept_id FROM system_users WHERE id = ? AND deleted = 0", userID).Scan(&deptID)

	var roles []struct {
		DataScope        int8   `gorm:"column:data_scope"`
		DataScopeDeptIDs string `gorm:"column:data_scope_dept_ids"`
		Code             string `gorm:"column:code"`
	}
	db.Raw(`SELECT r.data_scope, r.data_scope_dept_ids, r.code
		FROM system_role r JOIN system_user_role ur ON ur.role_id = r.id
		WHERE ur.user_id = ? AND r.status = 0 AND r.deleted = 0`, userID).Scan(&roles)

	dp := &contextx.DataPerm{UserID: userID}
	deptSet := map[int64]bool{}
	needSubtree := false
	for _, r := range roles {
		// 超级管理员 / 全部数据 → 直接放行。
		if r.Code == "super_admin" || r.DataScope == 1 {
			dp.All = true
			return dp, nil
		}
		switch r.DataScope {
		case 2: // 指定部门
			var ids []int64
			_ = json.Unmarshal([]byte(r.DataScopeDeptIDs), &ids)
			for _, id := range ids {
				deptSet[id] = true
			}
		case 3: // 本部门
			if deptID != 0 {
				deptSet[deptID] = true
			}
		case 4: // 本部门及以下
			if deptID != 0 {
				deptSet[deptID] = true
				needSubtree = true
			}
		case 5: // 仅本人
			dp.SelfOnly = true
		}
	}
	if needSubtree && deptID != 0 {
		for _, id := range s.descendants(db, deptID) {
			deptSet[id] = true
		}
	}
	for id := range deptSet {
		dp.DeptIDs = append(dp.DeptIDs, id)
	}
	return dp, nil
}

// descendants 返回 root 部门的全部后代部门编号（BFS）。
func (s *DataPermService) descendants(db *gorm.DB, root int64) []int64 {
	var depts []struct {
		ID       int64 `gorm:"column:id"`
		ParentID int64 `gorm:"column:parent_id"`
	}
	db.Raw("SELECT id, parent_id FROM system_dept WHERE deleted = 0").Scan(&depts)
	children := map[int64][]int64{}
	for _, d := range depts {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	var out []int64
	seen := map[int64]bool{root: true}
	queue := []int64{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range children[cur] {
			if !seen[ch] {
				seen[ch] = true
				out = append(out, ch)
				queue = append(queue, ch)
			}
		}
	}
	return out
}
