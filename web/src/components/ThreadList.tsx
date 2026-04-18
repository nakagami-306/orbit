import { useState } from 'react'
import type { EntityNode, TopicSummary } from '../api/client'

type FilterStatus = 'all' | 'open' | 'decided' | 'closed'

interface Props {
  threads: EntityNode[]
  topics: TopicSummary[]
  onSelectThread?: (threadId: string) => void
  onSelectTopic?: (topicId: string) => void
  selectedThreadId?: string | null
}

export default function ThreadList({ threads, topics, onSelectThread, onSelectTopic, selectedThreadId }: Props) {
  const [filter, setFilter] = useState<FilterStatus>('all')
  const [collapsedTopics, setCollapsedTopics] = useState<Set<string>>(new Set())

  const applyFilter = (list: EntityNode[]) =>
    filter === 'all' ? list : list.filter(t => t.status === filter)

  const sortThreads = (list: EntityNode[]) =>
    [...list].sort((a, b) => {
      const order: Record<string, number> = { open: 0, decided: 1, closed: 2 }
      const d = (order[a.status] ?? 3) - (order[b.status] ?? 3)
      if (d !== 0) return d
      return (b.instant || '').localeCompare(a.instant || '')
    })

  // Build topic -> threads mapping
  const topicThreadIds = new Set<string>()
  const topicGroups: { topic: TopicSummary; threads: EntityNode[] }[] = []

  for (const topic of topics) {
    const grouped = threads.filter(t => topic.threadIds.includes(t.id))
    for (const t of grouped) topicThreadIds.add(t.id)
    const filtered = applyFilter(grouped)
    if (filtered.length > 0 || filter === 'all') {
      topicGroups.push({ topic, threads: sortThreads(filtered) })
    }
  }

  const ungrouped = sortThreads(applyFilter(threads.filter(t => !topicThreadIds.has(t.id))))

  // Status counts
  const counts: Record<string, number> = {}
  for (const t of threads) counts[t.status] = (counts[t.status] || 0) + 1

  const filters: { key: FilterStatus; label: string }[] = [
    { key: 'all', label: 'All' },
    { key: 'open', label: 'Open' },
    { key: 'decided', label: 'Decided' },
    { key: 'closed', label: 'Closed' },
  ]

  const toggleCollapse = (topicId: string) => {
    setCollapsedTopics(prev => {
      const next = new Set(prev)
      next.has(topicId) ? next.delete(topicId) : next.add(topicId)
      return next
    })
  }

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem' }}>
      {/* Filter bar */}
      <div style={{
        display: 'flex', gap: '4px', marginBottom: '1rem',
        padding: '0 0 0.75rem', borderBottom: '1px solid #2a2a2a',
      }}>
        {filters.map(f => {
          const isActive = filter === f.key
          const count = f.key === 'all' ? threads.length : (counts[f.key] || 0)
          return (
            <button key={f.key} onClick={() => setFilter(f.key)} style={{
              background: isActive ? '#333' : 'transparent',
              border: isActive ? '1px solid #555' : '1px solid transparent',
              color: isActive ? '#e0e0e0' : '#888',
              fontSize: '0.75rem', padding: '4px 10px', borderRadius: '4px',
              cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px',
            }}
              onMouseEnter={e => { if (!isActive) e.currentTarget.style.background = '#2a2a2a' }}
              onMouseLeave={e => { if (!isActive) e.currentTarget.style.background = 'transparent' }}
            >
              {f.label}
              <span style={{ color: '#555', fontSize: '0.65rem' }}>({count})</span>
            </button>
          )
        })}
      </div>

      {/* Topic groups */}
      {topicGroups.map(({ topic, threads: groupThreads }) => {
        const isCollapsed = collapsedTopics.has(topic.id)
        const topicStatusColor = topic.status === 'open' ? '#a855f7' : '#666'

        return (
          <div key={topic.id} style={{
            marginBottom: '16px',
            border: '1px solid #2a2a2a',
            borderRadius: '8px',
            overflow: 'hidden',
          }}>
            {/* Topic header */}
            <div
              style={{
                padding: '10px 14px',
                background: '#1a1a2a',
                borderBottom: isCollapsed ? 'none' : '1px solid #2a2a2a',
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                cursor: 'pointer',
              }}
              onClick={() => toggleCollapse(topic.id)}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1 }}>
                <span style={{
                  fontSize: '0.6rem', fontWeight: 700, color: '#a855f7',
                  letterSpacing: '0.05em',
                }}>TOPIC</span>
                <span style={{ fontSize: '0.9rem', fontWeight: 600, color: '#e0e0e0' }}>
                  {topic.title}
                </span>
                <span style={{
                  fontSize: '0.6rem', padding: '1px 5px', borderRadius: '3px',
                  background: `${topicStatusColor}15`, color: topicStatusColor,
                }}>{topic.status}</span>
                <span style={{ fontSize: '0.65rem', color: '#555' }}>
                  ({groupThreads.length} thread{groupThreads.length !== 1 ? 's' : ''})
                </span>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                {onSelectTopic && (
                  <button
                    onClick={(e) => { e.stopPropagation(); onSelectTopic(topic.id) }}
                    style={{
                      fontSize: '0.65rem', padding: '2px 8px', borderRadius: '3px',
                      background: '#2a2a2a', border: '1px solid #444', color: '#888',
                      cursor: 'pointer',
                    }}
                    onMouseEnter={e => { e.currentTarget.style.color = '#ccc' }}
                    onMouseLeave={e => { e.currentTarget.style.color = '#888' }}
                  >detail</button>
                )}
                <span style={{
                  fontSize: '0.7rem', color: '#555',
                  transform: isCollapsed ? 'rotate(-90deg)' : 'rotate(0deg)',
                  transition: 'transform 0.15s', display: 'inline-block',
                }}>&#9660;</span>
              </div>
            </div>

            {/* Threads in topic */}
            {!isCollapsed && (
              <div style={{ padding: '6px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {groupThreads.map(thread => (
                  <ThreadRow key={thread.id} thread={thread} onSelect={onSelectThread} isSelected={selectedThreadId === thread.id} />
                ))}
                {groupThreads.length === 0 && (
                  <div style={{ padding: '8px 14px', fontSize: '0.8rem', color: '#555' }}>
                    No {filter} threads in this topic.
                  </div>
                )}
              </div>
            )}
          </div>
        )
      })}

      {/* Ungrouped threads */}
      {ungrouped.length > 0 && (
        <div>
          {topicGroups.length > 0 && (
            <div style={{
              fontSize: '0.7rem', color: '#555', textTransform: 'uppercase',
              letterSpacing: '0.05em', marginBottom: '8px', marginTop: '4px',
            }}>
              Ungrouped ({ungrouped.length})
            </div>
          )}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            {ungrouped.map(thread => (
              <ThreadRow key={thread.id} thread={thread} onSelect={onSelectThread} isSelected={selectedThreadId === thread.id} />
            ))}
          </div>
        </div>
      )}

      {threads.length === 0 && (
        <div style={{ textAlign: 'center', color: '#555', padding: '3rem', fontSize: '0.9rem' }}>
          No threads yet. Create one with <code style={{ color: '#888' }}>orbit thread create</code>.
        </div>
      )}
    </div>
  )
}

function ThreadRow({ thread, onSelect, isSelected }: {
  thread: EntityNode
  onSelect?: (id: string) => void
  isSelected: boolean
}) {
  const statusColor = thread.status === 'open' ? '#22c55e' : thread.status === 'decided' ? '#16a34a' : '#888'

  return (
    <div
      onClick={() => onSelect?.(thread.id)}
      style={{
        padding: '10px 12px',
        background: isSelected ? '#141c28' : '#1e1e1e',
        borderRadius: '6px',
        border: isSelected ? '1px solid #4a9eff' : '1px solid #333',
        borderLeft: `3px solid ${statusColor}`,
        transition: 'all 0.15s',
        cursor: onSelect ? 'pointer' : 'default',
      }}
      onMouseEnter={e => { if (!isSelected) e.currentTarget.style.background = '#222' }}
      onMouseLeave={e => { if (!isSelected) e.currentTarget.style.background = isSelected ? '#141c28' : '#1e1e1e' }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1 }}>
          <span style={{ width: '7px', height: '7px', borderRadius: '50%', background: statusColor, flexShrink: 0 }} />
          <span style={{ fontSize: '0.85rem', fontWeight: 500, color: '#e0e0e0' }}>{thread.title}</span>
        </div>
        <span style={{
          fontSize: '0.6rem', padding: '1px 5px', borderRadius: '3px',
          background: thread.status === 'open' ? '#0f2a1a' : '#252525',
          color: statusColor, flexShrink: 0, marginLeft: '8px',
        }}>{thread.status}</span>
      </div>
    </div>
  )
}
