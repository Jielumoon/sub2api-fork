import type { MonitorCoverage, MonitorMatrixBucket, MonitorMatrixRow } from '@/api/channelMonitorV2'

export interface BucketSlot {
  start: string
  bucket?: MonitorMatrixBucket
}

/**
 * 按 coverage 生成完整时间槽，口径与 RelayPulseMatrix 一致：
 * 起点向下对齐到桶宽，步进到 requested_end（旧响应缺省时用 data_through）。
 */
export function bucketStartsForCoverage(coverage: MonitorCoverage): string[] {
  const step = Math.max(60, coverage.bucket_seconds) * 1000
  const requestedStart = new Date(coverage.requested_start).getTime()
  const requestedEnd = coverage.requested_end ? new Date(coverage.requested_end).getTime() : NaN
  const end = Number.isFinite(requestedEnd) && requestedEnd > requestedStart
    ? requestedEnd
    : new Date(coverage.data_through).getTime()
  if (![requestedStart, end].every(Number.isFinite) || requestedStart >= end) return []
  const starts: string[] = []
  for (let cursor = Math.floor(requestedStart / step) * step; cursor < end; cursor += step) {
    starts.push(new Date(cursor).toISOString())
  }
  return starts
}

/** 后端只返回有流量的桶：按起点把每行的桶对位到时间槽上，空槽保留为无数据；索引表所有行共用。 */
export function alignRows<T extends MonitorMatrixRow>(
  rows: readonly T[],
  starts: string[],
): Array<{ row: T; slots: BucketSlot[] }> {
  const indexByStart = new Map(starts.map((start, index) => [start, index]))
  return rows.map((row) => {
    const slots: BucketSlot[] = starts.map((start) => ({ start }))
    for (const bucket of row.buckets || []) {
      const index = indexByStart.get(new Date(bucket.bucket_start).toISOString())
      if (index != null) slots[index] = { start: starts[index], bucket }
    }
    return { row, slots }
  })
}
