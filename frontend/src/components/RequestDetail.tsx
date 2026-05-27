import { useEffect, useState } from 'react'
import { Descriptions, Collapse, Spin, Tag, Typography, Grid, Modal } from '@arco-design/web-react'
import ReactJsonView from '@microlink/react-json-view'
import dayjs from 'dayjs'
import { getRequest } from '../api/client'
import type { APIRequestDetail } from '../types'

const { Item: CollapseItem } = Collapse
const { Row, Col } = Grid

interface Props {
  id: string
}

interface MessageModal {
  role: string
  content: string
}

const roleColor: Record<string, string> = {
  user: '#165dff',
  assistant: '#0fc6c2',
  system: '#7816ff',
  tool: '#ff7d00',
}

function statusColor(code: number): string {
  if (code >= 200 && code < 300) return 'green'
  if (code >= 400 && code < 500) return 'orange'
  return 'red'
}

function extractContent(raw: unknown): string {
  if (typeof raw === 'string') return raw
  if (Array.isArray(raw)) {
    return raw
      .filter((p: unknown) => p && typeof p === 'object' && (p as Record<string, unknown>).type === 'text')
      .map((p: unknown) => String((p as Record<string, unknown>).text ?? ''))
      .join('\n')
  }
  return JSON.stringify(raw, null, 2)
}

// Inline token types
type InlineToken =
  | { t: 'text'; s: string }
  | { t: 'code'; s: string }
  | { t: 'bold'; s: string }
  | { t: 'italic'; s: string }
  | { t: 'link'; label: string; href: string }

function tokenizeInline(text: string): InlineToken[] {
  const tokens: InlineToken[] = []
  let i = 0
  let plain = ''

  const flush = () => { if (plain) { tokens.push({ t: 'text', s: plain }); plain = '' } }

  while (i < text.length) {
    // inline code `...`
    if (text[i] === '`') {
      const end = text.indexOf('`', i + 1)
      if (end !== -1) { flush(); tokens.push({ t: 'code', s: text.slice(i + 1, end) }); i = end + 1; continue }
    }
    // bold **...**
    if (text.slice(i, i + 2) === '**') {
      const end = text.indexOf('**', i + 2)
      if (end !== -1) { flush(); tokens.push({ t: 'bold', s: text.slice(i + 2, end) }); i = end + 2; continue }
    }
    // italic *...* (not **)
    if (text[i] === '*' && text[i + 1] !== '*') {
      const end = text.indexOf('*', i + 1)
      if (end !== -1 && text[end + 1] !== '*') { flush(); tokens.push({ t: 'italic', s: text.slice(i + 1, end) }); i = end + 1; continue }
    }
    // link [label](href)
    if (text[i] === '[') {
      const cb = text.indexOf(']', i + 1)
      if (cb !== -1 && text[cb + 1] === '(') {
        const cp = text.indexOf(')', cb + 2)
        if (cp !== -1) { flush(); tokens.push({ t: 'link', label: text.slice(i + 1, cb), href: text.slice(cb + 2, cp) }); i = cp + 1; continue }
      }
    }
    plain += text[i++]
  }
  flush()
  return tokens
}

function InlineLine({ text }: { text: string }) {
  return (
    <>
      {tokenizeInline(text).map((tok, i) => {
        if (tok.t === 'code') return <code key={i} style={{ background: '#f2f3f5', padding: '1px 5px', borderRadius: 3, fontSize: 12, color: '#c7254e' }}>{tok.s}</code>
        if (tok.t === 'bold') return <strong key={i}>{tok.s}</strong>
        if (tok.t === 'italic') return <em key={i}>{tok.s}</em>
        if (tok.t === 'link') return <span key={i} style={{ color: '#4080ff' }}>[{tok.label}]({tok.href})</span>
        return <span key={i}>{tok.s}</span>
      })}
    </>
  )
}

function MarkdownContent({ content }: { content: string }) {
  const lines = content.split('\n')
  let inCodeBlock = false

  return (
    <div style={{ fontSize: 13, lineHeight: 1.75, fontFamily: 'inherit' }}>
      {lines.map((line, i) => {
        // code fence toggle
        if (line.startsWith('```')) {
          inCodeBlock = !inCodeBlock
          return <div key={i} style={{ color: '#86909c', fontFamily: 'monospace' }}>{line}</div>
        }
        if (inCodeBlock) {
          return <div key={i} style={{ fontFamily: 'monospace', background: '#f7f8fa', padding: '0 4px', color: '#333' }}>{line || ' '}</div>
        }
        // empty line → spacer
        if (line.trim() === '') return <div key={i} style={{ height: '0.6em' }} />
        // heading
        const hm = /^(#{1,6})\s+(.*)$/.exec(line)
        if (hm) {
          const sizes = [20, 17, 15, 14, 13, 13]
          const sz = sizes[hm[1].length - 1]
          return (
            <div key={i} style={{ fontSize: sz, fontWeight: 700, margin: '6px 0 2px', color: '#1d2129' }}>
              <span style={{ color: '#4080ff', marginRight: 4 }}>{hm[1]}</span>
              <InlineLine text={hm[2]} />
            </div>
          )
        }
        // blockquote
        if (line.startsWith('> ')) {
          return (
            <div key={i} style={{ borderLeft: '3px solid #c9cdd4', paddingLeft: 10, color: '#86909c', margin: '2px 0' }}>
              <span style={{ color: '#c9cdd4' }}>{'> '}</span>
              <InlineLine text={line.slice(2)} />
            </div>
          )
        }
        // unordered list
        const ulm = /^(\s*)([-*+])\s+(.*)$/.exec(line)
        if (ulm) {
          return (
            <div key={i} style={{ paddingLeft: ulm[1].length * 8 + 4, margin: '1px 0' }}>
              <span style={{ color: '#4080ff', marginRight: 6 }}>•</span>
              <InlineLine text={ulm[3]} />
            </div>
          )
        }
        // ordered list
        const olm = /^(\s*)(\d+)\.\s+(.*)$/.exec(line)
        if (olm) {
          return (
            <div key={i} style={{ paddingLeft: olm[1].length * 8 + 4, margin: '1px 0' }}>
              <span style={{ color: '#4080ff', marginRight: 6 }}>{olm[2]}.</span>
              <InlineLine text={olm[3]} />
            </div>
          )
        }
        // horizontal rule
        if (/^[-*_]{3,}$/.test(line.trim())) {
          return <hr key={i} style={{ border: 'none', borderTop: '1px solid #e5e6eb', margin: '8px 0' }} />
        }
        // normal line
        return <div key={i} style={{ margin: '1px 0' }}><InlineLine text={line} /></div>
      })}
    </div>
  )
}

export default function RequestDetailInline({ id }: Props) {
  const [detail, setDetail] = useState<APIRequestDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [msgModal, setMsgModal] = useState<MessageModal | null>(null)

  useEffect(() => {
    setLoading(true)
    getRequest(id)
      .then(setDetail)
      .finally(() => setLoading(false))
  }, [id])

  // fired when clicking any node in the req_body JSON viewer
  const handleReqSelect = (select: unknown) => {
    if (!detail) return
    const s = select as {
      namespace?: (string | number | null)[]
      name?: string | number | null
    }

    // Build full path by joining namespace + name so clicking on messages[n] itself also works
    const ns = (s.namespace ?? []).filter((x): x is string | number => x !== null && x !== undefined)
    if (s.name !== null && s.name !== undefined) ns.push(s.name as string | number)

    const messagesIdx = ns.indexOf('messages')
    if (messagesIdx === -1) return

    // Support both numeric index (0) and string index ("0") for array items
    const rawIdx = ns[messagesIdx + 1]
    if (rawIdx === undefined || rawIdx === null) return
    const msgIdx = typeof rawIdx === 'number' ? rawIdx : parseInt(String(rawIdx), 10)
    if (isNaN(msgIdx)) return

    const body = detail.req_body as Record<string, unknown> | undefined
    const messages = body?.messages as Record<string, unknown>[] | undefined
    const msg = messages?.[msgIdx]
    if (!msg) return

    const content = extractContent(msg.content)
    if (!content) return

    setMsgModal({ role: String(msg.role ?? 'unknown'), content })
  }

  if (loading) {
    return <div style={{ textAlign: 'center', padding: '24px 0' }}><Spin /></div>
  }
  if (!detail) return null

  const metaData = [
    { label: '请求 ID', value: <Typography.Text copyable style={{ fontSize: 12 }}>{detail.id}</Typography.Text> },
    { label: '时间', value: dayjs(detail.timestamp).format('YYYY-MM-DD HH:mm:ss') },
    { label: 'Endpoint', value: <Tag color="arcoblue" size="small">{detail.provider}</Tag> },
    { label: '模型', value: detail.model || '—' },
    { label: '状态码', value: <Tag color={statusColor(detail.status_code)} size="small">{detail.status_code}</Tag> },
    { label: '流式', value: <Tag color={detail.is_streaming ? 'cyan' : 'gray'} size="small">{detail.is_streaming ? '是' : '否'}</Tag> },
    { label: '耗时', value: `${detail.duration_ms} ms` },
    { label: 'Prompt Token', value: detail.prompt_tokens },
    { label: 'Completion Token', value: detail.completion_tokens },
    { label: '总 Token', value: detail.total_tokens },
    {
      label: '目标 URL',
      value: (
        <Typography.Text copyable style={{ fontSize: 12, wordBreak: 'break-all' }}>
          {detail.target_url}
        </Typography.Text>
      ),
    },
  ]

  return (
    <div style={{ padding: '8px 12px', background: '#fafafa', borderTop: '1px solid #e5e6eb' }}>
      <Descriptions
        data={metaData}
        border
        size="small"
        column={4}
        style={{ marginBottom: 8 }}
        labelStyle={{ fontSize: 12, color: '#86909c', padding: '4px 8px' }}
        valueStyle={{ fontSize: 12, padding: '4px 8px' }}
      />

      <Row gutter={8}>
        <Col span={12}>
          <Collapse bordered={false} style={{ background: '#fff' }}>
            <CollapseItem header="请求体（点击 messages 中的条目可预览内容）" name="req">
              <div style={{ maxHeight: 380, overflow: 'auto' }}>
                <ReactJsonView
                  src={detail.req_body as object || {}}
                  name={false}
                  collapsed={2}
                  enableClipboard
                  displayDataTypes={false}
                  style={{ fontSize: 12 }}
                  onSelect={handleReqSelect as (select: unknown) => void}
                />
              </div>
            </CollapseItem>
          </Collapse>
        </Col>
        <Col span={12}>
          <Collapse bordered={false} style={{ background: '#fff' }}>
            <CollapseItem header="响应体" name="resp">
              <div style={{ maxHeight: 380, overflow: 'auto' }}>
                <ReactJsonView
                  src={detail.resp_body as object || {}}
                  name={false}
                  collapsed={2}
                  enableClipboard
                  displayDataTypes={false}
                  style={{ fontSize: 12 }}
                />
              </div>
            </CollapseItem>
          </Collapse>
        </Col>
      </Row>

      <Collapse bordered={false} style={{ background: '#fff', marginTop: 4 }}>
        <CollapseItem header="请求头" name="headers">
          <div style={{ maxHeight: 200, overflow: 'auto' }}>
            {Object.entries(detail.req_headers || {}).map(([k, v]) => (
              <div key={k} style={{ display: 'flex', gap: 8, padding: '2px 0', fontSize: 12 }}>
                <span style={{ color: '#86909c', minWidth: 160, flexShrink: 0 }}>{k}</span>
                <span style={{ wordBreak: 'break-all' }}>{v}</span>
              </div>
            ))}
          </div>
        </CollapseItem>
      </Collapse>

      {/* Message content modal */}
      <Modal
        title={
          <span>
            <Tag
              color={roleColor[msgModal?.role ?? ''] ?? 'gray'}
              style={{ marginRight: 8, textTransform: 'capitalize' }}
            >
              {msgModal?.role}
            </Tag>
            消息内容
          </span>
        }
        visible={!!msgModal}
        onCancel={() => setMsgModal(null)}
        footer={null}
        style={{ width: 740, top: 60 }}
      >
        <div style={{ maxHeight: 'calc(80vh - 160px)', overflowY: 'auto', padding: '0 4px' }}>
          {msgModal && <MarkdownContent content={msgModal.content} />}
        </div>
      </Modal>
    </div>
  )
}
