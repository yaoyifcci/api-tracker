export interface APIRequestSummary {
  id: string
  timestamp: string
  provider: string
  model: string
  status_code: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  duration_ms: number
  is_streaming: boolean
  req_body: unknown
  resp_body: unknown
}

export interface StatsResponse {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
}

export interface APIRequestDetail extends APIRequestSummary {
  target_url: string
  method: string
  path: string
  req_headers: Record<string, string>
  resp_headers: Record<string, string>
}

export interface ListResponse {
  data: APIRequestSummary[]
  total: number
  page: number
  limit: number
}
