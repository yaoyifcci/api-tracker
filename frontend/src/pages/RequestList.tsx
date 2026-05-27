import { useEffect, useState, useCallback } from 'react'
import { Table, Tag, Badge, Button, Space, Typography, Tooltip } from '@arco-design/web-react'
import type { TableColumnProps } from '@arco-design/web-react'
import { IconRefresh, IconArrowUp, IconArrowDown, IconThunderbolt, IconSave } from '@arco-design/web-react/icon'
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


const tokenItems = [
  { icon: <IconArrowUp />,      color: '#165dff', key: 'prompt_tokens',     tip: '输入 Tokens' },
  { icon: <IconArrowDown />,    color: '#00b42a', key: 'completion_tokens', tip: '补全 Tokens' },
  { icon: <IconThunderbolt />,  color: '#ff7d00', key: 'cache_read_tokens', tip: '缓存读 Tokens' },
  { icon: <IconSave />,         color: '#722ed1', key: 'cache_write_tokens',tip: '缓存写 Tokens' },
] as const

function TokenDisplay({ row }: { row: APIRequestSummary }) {
  const hasAny = tokenItems.some(({ key }) => (row[key] ?? 0) > 0)
  if (!hasAny) return <span style={{ color: '#c9cdd4', fontSize: 12 }}>—</span>
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2px 6px' }}>
      {tokenItems.map(({ icon, color, key, tip }) => {
        const val = row[key] ?? 0
        return (
          <Tooltip key={key} content={tip} position="left">
            <span style={{ display: 'flex', alignItems: 'center', gap: 3, color: val > 0 ? color : '#c9cdd4', fontSize: 11, cursor: 'default', whiteSpace: 'nowrap' }}>
              {icon}
              <span style={{ fontVariantNumeric: 'tabular-nums' }}>{val}</span>
            </span>
          </Tooltip>
        )
      })}
    </div>
  )
}

function PreviewLines({ question, answer }: { question: string; answer: string }) {

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
        <span style={{ fontSize: 12, color: '#4e5969', whiteSpace: 'nowrap' }}>
          {dayjs(v).format('MM-DD HH:mm:ss')}
        </span>
      ),
    },
    {
      title: 'Endpoint',
      dataIndex: 'provider',
      width: 100,
      render: (v: string) => (
        <span style={{ whiteSpace: 'nowrap' }}>
          <Tag color={providerColors[v] || 'arcoblue'} size="small">{v}</Tag>
        </span>
      ),
    },
    {
      title: '模型',
      dataIndex: 'model',
      width: 160,
      ellipsis: true,
      render: (v: string) => (
        <Tooltip content={v} disabled={!v}>
          <span style={{ fontSize: 12, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', display: 'block' }}>
            {v || '—'}
          </span>
        </Tooltip>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status_code',
      width: 72,
      headerCellStyle: { whiteSpace: 'nowrap' },
      render: (v: number) => <span style={{ whiteSpace: 'nowrap' }}>{statusBadge(v)}</span>,
    },
    {
      title: 'Tokens',
      width: 160,
      headerCellStyle: { whiteSpace: 'nowrap' },
      render: (_: unknown, row: APIRequestSummary) => <TokenDisplay row={row} />,
    },
    {
      title: '耗时',
      dataIndex: 'duration_ms',
      width: 80,
      headerCellStyle: { whiteSpace: 'nowrap' },
      render: (v: number) => <span style={{ fontSize: 12, whiteSpace: 'nowrap' }}>{v}ms</span>,
    },
    {
      title: '流式',
      dataIndex: 'is_streaming',
      width: 60,
      headerCellStyle: { whiteSpace: 'nowrap' },
      render: (v: boolean) => (
        <span style={{ whiteSpace: 'nowrap' }}>
          <Tag color={v ? 'cyan' : 'gray'} size="small">{v ? 'SSE' : '—'}</Tag>
        </span>
      ),
    },
    {
      title: '对话预览',
      minWidth: 200,
      render: (_: unknown, row: APIRequestSummary) => (
        <PreviewLines question={row.preview_question} answer={row.preview_answer} />
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
        scroll={{ x: 1020 }}
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
