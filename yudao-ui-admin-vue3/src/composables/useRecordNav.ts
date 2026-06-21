// useRecordNav —— 列表 → 详情的「上一条/下一条记录」导航。
//
// 列表页把当前结果的 id 序列存进 sessionStorage(remember),
// 详情页据当前 id 求相邻 id(neighbors),实现 Axelor 风格的详情内翻记录,
// 无需回列表。sessionStorage 保证刷新/同标签内有效,跨标签不污染。
//
// key 约定用列表路由标识,如 'myerp:category'。

const PREFIX = 'axelor:nav:'

export function useRecordNav(key: string) {
  const storeKey = PREFIX + key

  function remember(ids: number[]): void {
    try {
      sessionStorage.setItem(storeKey, JSON.stringify(ids))
    } catch {
      /* ignore quota / 隐私模式 */
    }
  }

  function neighbors(id: number): { prev: number | null; next: number | null; index: number; total: number } {
    try {
      const ids = JSON.parse(sessionStorage.getItem(storeKey) || '[]') as number[]
      const i = ids.indexOf(id)
      if (i < 0) return { prev: null, next: null, index: -1, total: ids.length }
      return {
        prev: i > 0 ? ids[i - 1] : null,
        next: i < ids.length - 1 ? ids[i + 1] : null,
        index: i,
        total: ids.length
      }
    } catch {
      return { prev: null, next: null, index: -1, total: 0 }
    }
  }

  return { remember, neighbors }
}
