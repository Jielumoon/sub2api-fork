/** 按 key 分组，组与组内元素都保持首次出现的顺序。 */
export function groupBy<T, K>(items: readonly T[], key: (item: T) => K): Array<{ key: K; items: T[] }> {
  const groups = new Map<K, T[]>()
  for (const item of items) {
    const k = key(item)
    const list = groups.get(k)
    if (list) list.push(item)
    else groups.set(k, [item])
  }
  return [...groups].map(([k, list]) => ({ key: k, items: list }))
}
