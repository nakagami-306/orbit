import { useState } from 'react'
import type { EntityNode } from '../api/client'

type FilterStatus = 'all' | 'open' | 'decided' | 'closed'

interface Props {
  threads: EntityNode[]
  onSelectThread?: (threadId: string) => void
  selectedThreadId?: string | null
}

export default function ThreadList({ threads, onSelectThread, selectedThreadId }: Props) {
  const [filter, setFilter] = useState<FilterStatus>('all')

  const filtered = filter === 'all'
    ? threads
    : threads.filter(t => t.status === filter)

  // Sort: open first, then by instant desc
  const sorted = [...filtered].sort((a, b) => {
    const statusOrder: Record<string, number> = { open: 0, decided: 1, closed: 2 }
    const aOrder = statusOrder[a.status] ?? 3
    const bOrder = statusOrder[b.status] ?? 3
    if (aOrder !== bOrder) return aOrder - bOrder
    return (b.instant || '').localeCompare(a.instant || '')
  })

  const filters: { key: FilterStatus; label: string }[] = [
    { key: 'all', label: 'All' },
    { key: 'open', label: 'Open' },
    { key: 'decided', label: 'Decided' },
    { key: 'closed', label: 'Closed' },
  ]

  // Count by status
  const counts: Record<string, number> = {}
  for (const t of threads) {
    counts[t.status] = (counts[t.status] || 0) + 1
  }

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem' }}>
      {/* Filter bar */}
      <div style={{
        display: 'flex',
        gap: '4px',
        marginBottom: '1rem',
        padding: '0 0 0.75rem',
        borderBottom: '1px solid #2a2a2a',
      }}>
        {filters.map(f => {
          const isActive = filter === f.key
          const count = f.key === 'all' ? threads.length : (counts[f.key] || 0)
          return (
            <button
              key={f.key}
              onClick={() => setFilter(f.key)}
              style={{
                background: isActive ? '#333' : 'transparent',
                border: isActive ? '1px solid #555' : '1px solid transparent',
                color: isActive ? '#e0e0e0' : '#888',
                fontSize: '0.75rem',
                padding: '4px 10px',
                borderRadius: '4px',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
              }}
              onMouseEnter={e => {
                if (!isActive) e.currentTarget.style.background = '#2a2a2a'
              }}
              onMouseLeave={e => {
                if (!isActive) e.currentTarget.style.background = 'transparent'
              }}
            >
              {f.label}
              <span style={{ color: '#555', fontSize: '0.65rem' }}>({count})</span>
            </button>
          )
        })}
      </div>

      {/* Thread list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {sorted.map(thread => {
          const statusColor = thread.status === 'open'
            ? '#22c55e'
            : thread.status === 'decided'
              ? '#16a34a'
              : '#888'

          const isSelected = selectedThreadId === thread.id
          return (
            <div
              key={thread.id}
              onClick={() => onSelectThread?.(thread.id)}
              style={{
                padding: '12px 14px',
                background: isSelected ? '#141c28' : '#1e1e1e',
                borderRadius: '8px',
                border: isSelected ? '1px solid #4a9eff' : '1px solid #333',
                borderLeft: `3px solid ${statusColor}`,
                transition: 'all 0.15s',
                cursor: onSelectThread ? 'pointer' : 'default',
              }}
              onMouseEnter={e => {
                if (!isSelected) {
                  e.currentTarget.style.borderRightColor = '#555'
                  e.currentTarget.style.borderTopColor = '#555'
                  e.currentTarget.style.borderBottomColor = '#555'
                }
              }}
              onMouseLeave={e => {
                if (!isSelected) {
                  e.currentTarget.style.borderRightColor = '#333'
                  e.currentTarget.style.borderTopColor = '#333'
                  e.currentTarget.style.borderBottomColor = '#333'
                }
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div style={{ flex: 1 }}>
                  <div style={{
                    fontSize: '0.9rem',
                    fontWeight: 500,
                    color: '#e0e0e0',
                    marginBottom: '4px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                  }}>
                    <span style={{
                      width: '8px',
                      height: '8px',
                      borderRadius: '50%',
                      background: statusColor,
                      flexShrink: 0,
                    }} />
                    {thread.title}
                  </div>
                </div>

                <span style={{
                  fontSize: '0.65rem',
                  padding: '2px 6px',
                  borderRadius: '3px',
                  background: thread.status === 'open' ? '#0f2a1a' : '#252525',
                  color: statusColor,
                  flexShrink: 0,
                  marginLeft: '8px',
                }}>
                  {thread.status}
                </span>
              </div>

              <div style={{
                fontSize: '0.7rem',
                color: '#555',
                marginTop: '4px',
                marginLeft: '16px',
                fontFamily: 'monospace',
              }}>
                {thread.id.slice(0, 8)}
                {thread.instant && (
                  <span style={{ marginLeft: '12px' }}>
                    {thread.instant.slice(0, 19).replace('T', ' ')}
                  </span>
                )}
              </div>
            </div>
          )
        })}
      </div>

      {sorted.length === 0 && (
        <div style={{
          textAlign: 'center',
          color: '#555',
          padding: '3rem',
          fontSize: '0.9rem',
        }}>
          {filter === 'all'
            ? <>No threads yet. Create one with <code style={{ color: '#888' }}>orbit thread create</code>.</>
            : `No ${filter} threads.`
          }
        </div>
      )}
    </div>
  )
}
