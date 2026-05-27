import { useEffect, useState, useCallback } from 'react'
import { Table, Tag, Badge, Button, Space, Typography, Tooltip } from '@arco-design/web-react'
import type { TableColumnProps } from '@arco-design/web-react'
import { IconRefresh } from '@arco-design/web-react/icon'
import dayjs from 'dayjs'
import { listRequests } from '../api/client'
import type { APIRequestSummary } from '../types'
import RequestDetailInline from '../components/RequestDetail'

const providerColors: Record<string, string> = {
  openai: 'green',
  anthropic: 'purple',
  openai_responses: 'blue',
}

function statusBadge(code: number) {
  if (code >= 200 && code < 300) return <Badge status="success" text={String(code)} />
  if (code >= 400 && code < 500) return <Badge status="warning" text={String(code)} />
  return <Badge status="error" text={String(code)} />
}

function extractLastUserMessage(reqBody: unknown): string {
  if (!reqBody || typeof reqBody !== 'object') return ''
  const body = reqBody as Record<string, unknown>
  const messages = body.messages
  if (!Array.isArray(messages)) return ''
  const userMsgs = messages.filter(
    (m: unknown) => m && typeof m === 'object' && (m as Record<string, unknown>).role === 'user'
  )
  if (userMsgs.length === 0) return ''
  const last = userMsgs[userMsgs.length - 1] as Record<string, unknown>
  const content = last.content
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    const textPart = content.find(
      (p: unknown) => p && typeof p === 'object' && (p as Record<string, unknown>).type === 'text'
    ) as Record<string, unknown> | undefined
    if (textPart) return String(textPart.text ?? '')
  }
  return JSON.stringify(content)
}

function extractAssistantResponse(respBody: unknown): string {
  if (!respBody || typeof respBody !== 'object') return ''
  const body = respBody as Record<string, unknown>

  // OpenAI chat completions: choices[0].message.content
  if (Array.isArray(body.choices) && body.choices.length > 0) {
    const choice = body.choices[0] as Record<string, unknown>
    const message = choice.message as Record<string, unknown> | undefined
    if (message && typeof message.content === 'string') return message.content
  }

  // Anthropic aggregated: content[0].text
  if (Array.isArray(body.content) && body.content.length > 0) {
    const part = body.content[0] as Record<string, unknown>
    if (part.type === 'text' && typeof part.text === 'string') return part.text
  }

  // OpenAI Responses API: content (string)
  if (typeof body.content === 'string') return body.content

  return ''
}

function PreviewLines({ reqBody, respBody }: { reqBody: unknown; respBody: unknown }) {
  const question = extractLastUserMessage(reqBody)
  const answer = extractAssistantResponse(respBody)

  const lineStyle: React.CSSProperties = {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    fontSize: 12,
    lineHeight: '18px',
  }
  const labelStyle: React.CSSProperties = {
    display: 'inline-block',
    width: 16,
    color: '#c9cdd4',
    flexShrink: 0,
    fontSize: 11,
    fontWeight: 600,
  }

  return (
    <div style={{ minWidth: 0 }}>
      <Tooltip
        content={question ? <span style={{ whiteSpace: 'pre-wrap', display: 'block', maxWidth: 500 }}>{question}</span> : undefined}
        disabled={!question}
        position="top"
      >
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, ...lineStyle }}>
          <span style={labelStyle}>Q</span>
          <span style={{ color: '#1d2129', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {question || <span style={{ color: '#c9cdd4' }}>—</span>}
          </span>
        </div>
      </Tooltip>
      <Tooltip
        content={answer ? <span style={{ whiteSpace: 'pre-wrap', display: 'block', maxWidth: 500 }}>{answer}</span> : undefined}
        disabled={!answer}
        position="bottom"
      >
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, ...lineStyle, marginTop: 2 }}>
          <span style={labelStyle}>A</span>
          <span style={{ color: '#86909c', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {answer || <span style={{ color: '#c9cdd4' }}>—</span>}
          </span>
        </div>
      </Tooltip>
    </div>
  )
}

export default function RequestList() {
  const [data, setData] = useState<APIRequestSummary[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [limit] = useState(20)
  const [loading, setLoading] = useState(false)
  const [expandedKeys, setExpandedKeys] = useState<string[]>([])

  const fetchData = useCallback(async (p: number) => {
    setLoading(true)
    try {
      const res = await listRequests(p, limit)
      setData(res.data)
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [limit])

  useEffect(() => { fetchData(page) }, [page, fetchData])

  useEffect(() => {
    if (page !== 1) return
    const timer = setInterval(() => fetchData(1), 10000)
    return () => clearInterval(timer)
  }, [page, fetchData])

  const toggleExpand = (id: string) => {
    setExpandedKeys(prev =>
      prev.includes(id) ? prev.filter(k => k !== id) : [...prev, id]
    )
  }

  const columns: TableColumnProps<APIRequestSummary>[] = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      width: 130,
      render: (v: string) => (
        <span style={{ fontSize: 12, color: '#4e5969' }}>
          {dayjs(v).format('MM-DD HH:mm:ss')}
        </span>
      ),
    },
    {
      title: 'Endpoint',
      dataIndex: 'provider',
      width: 90,
      render: (v: string) => (
        <Tag color={providerColors[v] || 'arcoblue'} size="small">{v}</Tag>
      ),
    },
    {
      title: '模型',
      dataIndex: 'model',
      width: 120,
      render: (v: string) => (
        <span style={{ fontSize: 12 }}>{v || '—'}</span>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status_code',
      width: 72,
      render: (v: number) => <span style={{ whiteSpace: 'nowrap' }}>{statusBadge(v)}</span>,
    },
    {
      title: 'Tokens',
      dataIndex: 'total_tokens',
      width: 70,
      render: (v: number) => <span style={{ fontSize: 12 }}>{v || '—'}</span>,
    },
    {
      title: '耗时',
      dataIndex: 'duration_ms',
      width: 70,
      render: (v: number) => <span style={{ fontSize: 12 }}>{v}ms</span>,
    },
    {
      title: '流式',
      dataIndex: 'is_streaming',
      width: 52,
      render: (v: boolean) => (
        <Tag color={v ? 'cyan' : 'gray'} size="small">{v ? 'SSE' : '—'}</Tag>
      ),
    },
    {
      title: '对话预览',
      render: (_: unknown, row: APIRequestSummary) => (
        <PreviewLines reqBody={row.req_body} respBody={row.resp_body} />
      ),
    },
  ]

  return (
    <div style={{ padding: '10px 16px' }}>
      <Space style={{ marginBottom: 8 }}>
        <Typography.Title heading={6} style={{ margin: 0 }}>API Tracker</Typography.Title>
        <Button
          size="small"
          icon={<IconRefresh />}
          loading={loading}
          onClick={() => fetchData(page)}
        >
          刷新
        </Button>
      </Space>
      <Table
        columns={columns}
        data={data}
        loading={loading}
        rowKey="id"
        size="small"
        expandedRowKeys={expandedKeys}
        onExpandedRowsChange={(keys) => setExpandedKeys(keys as string[])}
        expandedRowRender={(record: APIRequestSummary) => (
          <div onClick={(e) => e.stopPropagation()}>
            <RequestDetailInline id={record.id} />
          </div>
        )}
        pagination={{
          total,
          current: page,
          pageSize: limit,
          showTotal: (t) => `共 ${t} 条`,
          onChange: setPage,
        }}
        onRow={(row) => ({
          onClick: () => toggleExpand(row.id),
          style: { cursor: 'pointer' },
        })}
        stripe
        hover
      />
    </div>
  )
}
