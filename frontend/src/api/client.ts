import axios from 'axios'
import type { APIRequestDetail, EndpointInfo, ListFilterParams, ListResponse, StatsResponse } from '../types'

const http = axios.create({ baseURL: '/api' })

export async function listRequests(
  page: number,
  limit: number,
  filter: ListFilterParams = {},
): Promise<ListResponse> {
  const params: Record<string, string | number> = { page, limit }
  for (const [k, v] of Object.entries(filter)) {
    if (v !== undefined && v !== null && v !== '') params[k] = v as string | number
  }
  const { data } = await http.get<ListResponse>('/requests', { params })
  return data
}

export async function listEndpoints(): Promise<EndpointInfo[]> {
  const { data } = await http.get<{ data: EndpointInfo[] }>('/endpoints')
  return data.data
}

export async function getRequest(id: string): Promise<APIRequestDetail> {
  const { data } = await http.get<APIRequestDetail>(`/requests/${id}`)
  return data
}

export async function getStats(): Promise<StatsResponse> {
  const { data } = await http.get<StatsResponse>('/stats')
  return data
}
