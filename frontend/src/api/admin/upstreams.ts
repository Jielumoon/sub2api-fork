/**
 * 管理端上游实体 API
 */

import { apiClient } from '../client'
import type {
  Upstream,
  CreateUpstreamRequest,
  UpdateUpstreamRequest,
  UpstreamProbeResponse,
  PaginatedResponse
} from '@/types'

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: { search?: string },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<Upstream>> {
  const { data } = await apiClient.get<PaginatedResponse<Upstream>>('/admin/upstreams', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  if (!Array.isArray(data?.items)) {
    throw new Error('Invalid upstream list response')
  }
  return data
}

export async function getById(id: number): Promise<Upstream> {
  const { data } = await apiClient.get<Upstream>(`/admin/upstreams/${id}`)
  return data
}

export async function create(payload: CreateUpstreamRequest): Promise<Upstream> {
  const { data } = await apiClient.post<Upstream>('/admin/upstreams', payload)
  return data
}

export async function update(id: number, payload: UpdateUpstreamRequest): Promise<Upstream> {
  const { data } = await apiClient.put<Upstream>(`/admin/upstreams/${id}`, payload)
  return data
}

export async function deleteUpstream(id: number): Promise<{ id: number }> {
  const { data } = await apiClient.delete<{ id: number }>(`/admin/upstreams/${id}`)
  return data
}

// 手动探测：忽略 enabled 与到期时间
export async function probe(id: number): Promise<UpstreamProbeResponse> {
  const { data } = await apiClient.post<UpstreamProbeResponse>(`/admin/upstreams/${id}/probe`)
  return data
}

export const upstreamsAPI = {
  list,
  getById,
  create,
  update,
  delete: deleteUpstream,
  probe
}

export default upstreamsAPI
