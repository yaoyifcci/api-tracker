import axios from 'axios'
import type { APIRequestDetail, ListResponse, StatsResponse } from '../types'

const http = axios.create({ baseURL: '/api' })

export async function listRequests(page: number, limit: number): Promise<ListResponse> {
  const { data } = await http.get<ListResponse>('/requests', { params: { page, limit } })
  return data
}

export async function getRequest(id: string): Promise<APIRequestDetail> {
  const { data } = await http.get<APIRequestDetail>(`/requests/${id}`)
  return data
}

export async function getStats(): Promise<StatsResponse> {
  const { data } = await http.get<StatsResponse>('/stats')
  return data
}
