import { useEffect, useState } from 'react'
import { getStats } from '../api/client'
import type { StatsResponse } from '../types'

function fmt(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'k'
  return String(n)
}

interface StatItemProps {
  label: string
  value: number
  color?: string
}

function StatItem({ label, value, color = '#c9cdd4' }: StatItemProps) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      <span style={{ color, fontSize: 11, fontWeight: 600, letterSpacing: '0.02em' }}>{label}</span>
      <span style={{ color: '#fff', fontSize: 12, fontWeight: 500, fontVariantNumeric: 'tabular-nums' }}>
        {fmt(value)}
      </span>
    </div>
  )
}

const DIVIDER = <span style={{ color: '#4e5969', fontSize: 12 }}>|</span>

export default function TokenStatsFooter() {
  const [stats, setStats] = useState<StatsResponse | null>(null)

  const load = () => {
    getStats().then(setStats).catch(() => {})
  }

  useEffect(() => {
    load()
    const timer = setInterval(load, 30_000)
    return () => clearInterval(timer)
  }, [])

  if (!stats) return null

  return (
    <div style={{
      position: 'sticky',
      bottom: 0,
      height: 32,
      background: '#1d2129',
      borderTop: '1px solid #2a2d35',
      display: 'flex',
      alignItems: 'center',
      gap: 12,
      padding: '0 16px',
      zIndex: 100,
    }}>
      <span style={{ color: '#4e5969', fontSize: 11, fontWeight: 600, marginRight: 4 }}>TOKENS</span>
      <StatItem label="Input" value={stats.prompt_tokens} color="#7bc3ff" />
      {DIVIDER}
      <StatItem label="Completion" value={stats.completion_tokens} color="#7be8d3" />
      {DIVIDER}
      <StatItem label="Cache Read" value={stats.cache_read_tokens} color="#ffd666" />
      {DIVIDER}
      <StatItem label="Cache Write" value={stats.cache_write_tokens} color="#ff9a4d" />
      {DIVIDER}
      <StatItem label="Total" value={stats.total_tokens} color="#c9cdd4" />
    </div>
  )
}
