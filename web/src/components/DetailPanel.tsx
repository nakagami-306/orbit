import { useEffect, useState } from 'react'
import { fetchJSON, type DecisionDetail, type ThreadDetail, type TopicDetail } from '../api/client'
import { formatTimeFull } from '../utils/time'

export type PanelTarget =
  | { kind: 'decision'; id: string; branch?: string }
  | { kind: 'thread'; id: string }
  | { kind: 'topic'; id: string }

interface Props {
  projectId: string
  branch?: string
  target: PanelTarget
  onClose: () => void
  onOpenThread?: (threadId: string) => void
}

export default function DetailPanel({ projectId, branch, target, onClose, onOpenThread }: Props) {
  return (
    <div style={{
      width: '420px',
      borderLeft: '1px solid #333',
      background: '#1e1e1e',
      padding: '1rem',
      overflow: 'auto',
      flexShrink: 0,
      height: '100%',
    }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{
          margin: 0, fontSize: '0.85rem', textTransform: 'uppercase', letterSpacing: '0.05em',
          color: target.kind === 'thread' ? '#22c55e' : target.kind === 'topic' ? '#a855f7' : '#888',
        }}>
          {target.kind === 'decision' ? 'Decision Detail' : target.kind === 'thread' ? 'Thread Detail' : 'Topic Detail'}
        </h3>
        <button onClick={onClose} style={{
          background: '#333', border: 'none', color: '#888', cursor: 'pointer',
          fontSize: '0.75rem', padding: '4px 8px', borderRadius: '4px',
        }}
          onMouseEnter={e => { e.currentTarget.style.background = '#444' }}
          onMouseLeave={e => { e.currentTarget.style.background = '#333' }}
        >
          ESC
        </button>
      </div>

      {target.kind === 'decision' && <DecisionContent projectId={projectId} branch={target.branch ?? branch} decisionId={target.id} onOpenThread={onOpenThread} />}
      {target.kind === 'thread' && <ThreadContent projectId={projectId} threadId={target.id} />}
      {target.kind === 'topic' && <TopicContent projectId={projectId} topicId={target.id} onOpenThread={onOpenThread} />}
    </div>
  )
}

// --- Decision Content ---

function DecisionContent({ projectId, branch, decisionId, onOpenThread }: {
  projectId: string; branch?: string; decisionId: string; onOpenThread?: (id: string) => void
}) {
  const [detail, setDetail] = useState<DecisionDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [expandedChange, setExpandedChange] = useState<number | null>(null)

  useEffect(() => {
    setError('')
    setLoading(true)
    setExpandedChange(null)
    const branchParam = branch ? `?branch=${encodeURIComponent(branch)}` : ''
    fetchJSON<DecisionDetail>(`/api/projects/${projectId}/decisions/${decisionId}${branchParam}`)
      .then(setDetail)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [projectId, branch, decisionId])

  if (loading) return <div style={{ color: '#888', fontSize: '0.85rem' }}>Loading...</div>
  if (error) return <div style={{ color: '#f66', fontSize: '0.85rem' }}>Error: {error}</div>
  if (!detail) return null

  return (
    <>
      <h4 style={{ margin: '0 0 0.5rem', fontSize: '1.05rem', lineHeight: 1.3, color: '#e0e0e0' }}>
        {detail.title}
      </h4>
      <div style={{ fontSize: '0.8rem', color: '#888', marginBottom: '0.5rem', display: 'flex', gap: '6px' }}>
        <span>{detail.author}</span>
        <span style={{ color: '#555' }}>·</span>
        <span>{formatTimeFull(detail.instant)}</span>
      </div>
      <div style={{ fontSize: '0.7rem', color: '#555', marginBottom: '1rem', fontFamily: 'monospace' }}>
        {detail.id}
      </div>

      {detail.rationale && (
        <InfoSection title="Rationale">
          <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc', lineHeight: 1.5 }}>{detail.rationale}</p>
        </InfoSection>
      )}
      {detail.context && (
        <InfoSection title="Context">
          <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc', lineHeight: 1.5 }}>{detail.context}</p>
        </InfoSection>
      )}

      {detail.sourceThread && (
        <InfoSection title="Source Thread">
          <div
            onClick={() => onOpenThread?.(detail.sourceThread!.id)}
            style={{
              fontSize: '0.85rem', padding: '8px 10px', background: '#252525',
              borderRadius: '4px', borderLeft: '3px solid #22c55e',
              cursor: onOpenThread ? 'pointer' : 'default',
            }}
            onMouseEnter={e => { if (onOpenThread) e.currentTarget.style.background = '#2a2a2a' }}
            onMouseLeave={e => { e.currentTarget.style.background = '#252525' }}
          >
            <div style={{ color: '#e0e0e0' }}>{detail.sourceThread.title}</div>
            <div style={{ fontSize: '0.7rem', color: '#888', marginTop: '2px', display: 'flex', alignItems: 'center', gap: '6px' }}>
              <StatusBadge status={detail.sourceThread.status} color="#22c55e" />
              {onOpenThread && <span style={{ color: '#555' }}>click to view entries</span>}
            </div>
          </div>
        </InfoSection>
      )}

      {detail.changes.length > 0 && (
        <InfoSection title={`Changes (${detail.changes.length})`}>
          {detail.changes
            .filter(c => c.attribute === 'section/content' || c.attribute === 'section/title')
            .map((c, i) => {
              const isAdd = c.before === null
              const isDel = c.after === null
              const isModify = !isAdd && !isDel
              const color = isAdd ? '#4c4' : isDel ? '#f44' : '#fa4'
              const label = isAdd ? 'added' : isDel ? 'removed' : 'modified'
              const isExpanded = expandedChange === i
              return (
                <div key={i} style={{ marginBottom: '6px' }}>
                  <div onClick={() => setExpandedChange(isExpanded ? null : i)} style={{
                    fontSize: '0.8rem', padding: '6px 10px', background: '#252525',
                    borderRadius: '4px', borderLeft: `3px solid ${color}`, cursor: 'pointer',
                    display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                  }}
                    onMouseEnter={e => { e.currentTarget.style.background = '#2a2a2a' }}
                    onMouseLeave={e => { e.currentTarget.style.background = '#252525' }}
                  >
                    <span style={{ color: '#888' }}>{c.attribute.split('/').pop()}</span>
                    <span style={{ fontSize: '0.65rem', color }}>{label}</span>
                  </div>
                  {isExpanded && (isModify || isAdd || isDel) && (
                    <div style={{ fontSize: '0.75rem', margin: '4px 0 0', borderRadius: '4px', overflow: 'hidden', border: '1px solid #333' }}>
                      {c.before !== null && (
                        <div style={{ padding: '6px 10px', background: '#2a1515', color: '#e88', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: '200px', overflow: 'auto' }}>
                          <span style={{ color: '#f44', marginRight: '6px' }}>-</span>{truncate(c.before, 500)}
                        </div>
                      )}
                      {c.after !== null && (
                        <div style={{ padding: '6px 10px', background: '#152a15', color: '#8e8', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: '200px', overflow: 'auto' }}>
                          <span style={{ color: '#4c4', marginRight: '6px' }}>+</span>{truncate(c.after, 500)}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          {detail.changes.filter(c => c.attribute !== 'section/content' && c.attribute !== 'section/title').map((c, i) => (
            <div key={`other-${i}`} style={{ fontSize: '0.75rem', padding: '4px 10px', color: '#666' }}>
              {c.entityType}/{c.attribute.split('/').pop()}
            </div>
          ))}
        </InfoSection>
      )}

      {detail.relatedTasks.length > 0 && (
        <InfoSection title="Related Tasks">
          {detail.relatedTasks.map(t => (
            <div key={t.id} style={{
              fontSize: '0.85rem', padding: '6px 10px', margin: '4px 0', background: '#252525',
              borderRadius: '4px', display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}>
              <span style={{ color: '#ccc' }}>{t.title}</span>
              <StatusBadge status={t.status} color="#f97316" />
            </div>
          ))}
        </InfoSection>
      )}
    </>
  )
}

// --- Thread Content ---

const entryTypeConfig: Record<string, { color: string; label: string }> = {
  option: { color: '#4a9eff', label: 'OPTION' },
  finding: { color: '#22c55e', label: 'FINDING' },
  argument: { color: '#f97316', label: 'ARGUMENT' },
  conclusion: { color: '#a855f7', label: 'CONCLUSION' },
  note: { color: '#888', label: 'NOTE' },
}

function ThreadContent({ projectId, threadId }: { projectId: string; threadId: string }) {
  const [detail, setDetail] = useState<ThreadDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setError('')
    setLoading(true)
    fetchJSON<ThreadDetail>(`/api/projects/${projectId}/threads/${threadId}`)
      .then(setDetail)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [projectId, threadId])

  if (loading) return <div style={{ color: '#888', fontSize: '0.85rem' }}>Loading...</div>
  if (error) return <div style={{ color: '#f66', fontSize: '0.85rem' }}>Error: {error}</div>
  if (!detail) return null

  const statusColor = detail.status === 'open' ? '#22c55e' : detail.status === 'decided' ? '#16a34a' : '#888'

  return (
    <>
      <h4 style={{ margin: '0 0 0.5rem', fontSize: '1.05rem', lineHeight: 1.3, color: '#e0e0e0' }}>
        {detail.title}
      </h4>
      <div style={{ marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '8px' }}>
        <StatusBadge status={detail.status} color={statusColor} />
        <span style={{ fontSize: '0.7rem', color: '#555', fontFamily: 'monospace' }}>{detail.id}</span>
      </div>

      {detail.question && (
        <InfoSection title="Question">
          <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc', lineHeight: 1.5, fontStyle: 'italic' }}>
            {detail.question}
          </p>
        </InfoSection>
      )}

      <InfoSection title={`Entries (${detail.entries.length})`}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
          {detail.entries.map(entry => {
            const config = entryTypeConfig[entry.type] || entryTypeConfig.note
            return (
              <div key={entry.id} style={{
                padding: '8px 10px',
                background: '#252525',
                borderRadius: '4px',
                borderLeft: `3px solid ${config.color}`,
                opacity: entry.isRetracted ? 0.4 : 1,
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                  <span style={{
                    fontSize: '0.6rem', fontWeight: 700, color: config.color,
                    letterSpacing: '0.05em',
                  }}>
                    {config.label}
                    {entry.stance && <span style={{ marginLeft: '6px', color: '#888', fontWeight: 400 }}>({entry.stance})</span>}
                  </span>
                  <span style={{ fontSize: '0.6rem', color: '#555' }}>
                    {entry.author} · {entry.instant.slice(11, 16)}
                  </span>
                </div>
                <div style={{
                  fontSize: '0.8rem', color: '#ccc', lineHeight: 1.5,
                  whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                }}>
                  {entry.isRetracted ? <span style={{ color: '#666', fontStyle: 'italic' }}>(retracted)</span> : entry.content}
                </div>
              </div>
            )
          })}
        </div>
      </InfoSection>
    </>
  )
}

// --- Topic Content ---

function TopicContent({ projectId, topicId, onOpenThread }: {
  projectId: string; topicId: string; onOpenThread?: (id: string) => void
}) {
  const [detail, setDetail] = useState<TopicDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setError('')
    setLoading(true)
    fetchJSON<TopicDetail>(`/api/projects/${projectId}/topics/${topicId}`)
      .then(setDetail)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [projectId, topicId])

  if (loading) return <div style={{ color: '#888', fontSize: '0.85rem' }}>Loading...</div>
  if (error) return <div style={{ color: '#f66', fontSize: '0.85rem' }}>Error: {error}</div>
  if (!detail) return null

  const statusColor = detail.status === 'open' ? '#a855f7' : '#666'

  return (
    <>
      <h4 style={{ margin: '0 0 0.5rem', fontSize: '1.05rem', lineHeight: 1.3, color: '#e0e0e0' }}>
        {detail.title}
      </h4>
      <div style={{ marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '8px' }}>
        <StatusBadge status={detail.status} color={statusColor} />
        <span style={{ fontSize: '0.7rem', color: '#555', fontFamily: 'monospace' }}>{detail.id}</span>
      </div>

      {detail.description && (
        <InfoSection title="Description">
          <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc', lineHeight: 1.5 }}>
            {detail.description}
          </p>
        </InfoSection>
      )}

      <InfoSection title={`Threads (${detail.threads.length})`}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
          {detail.threads.map(t => {
            const color = t.status === 'open' ? '#22c55e' : t.status === 'decided' ? '#16a34a' : '#888'
            return (
              <div
                key={t.id}
                onClick={() => onOpenThread?.(t.id)}
                style={{
                  padding: '8px 10px', background: '#252525', borderRadius: '4px',
                  borderLeft: `3px solid ${color}`,
                  cursor: onOpenThread ? 'pointer' : 'default',
                }}
                onMouseEnter={e => { if (onOpenThread) e.currentTarget.style.background = '#2a2a2a' }}
                onMouseLeave={e => { e.currentTarget.style.background = '#252525' }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '0.85rem', color: '#e0e0e0' }}>{t.title}</span>
                  <StatusBadge status={t.status} color={color} />
                </div>
                {t.question && (
                  <div style={{ fontSize: '0.75rem', color: '#888', marginTop: '4px', fontStyle: 'italic' }}>
                    {t.question.length > 80 ? t.question.slice(0, 80) + '...' : t.question}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </InfoSection>
    </>
  )
}

// --- Shared components ---

function InfoSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: '1rem' }}>
      <div style={{ fontSize: '0.7rem', color: '#666', marginBottom: '6px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
        {title}
      </div>
      {children}
    </div>
  )
}

function StatusBadge({ status, color }: { status: string; color: string }) {
  return (
    <span style={{
      fontSize: '0.65rem', padding: '1px 6px', borderRadius: '3px',
      background: `${color}15`, color,
    }}>
      {status}
    </span>
  )
}

function truncate(s: string, max: number): string {
  return s.length <= max ? s : s.slice(0, max) + '...'
}
