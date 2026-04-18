import { useEffect, useState } from 'react'
import { fetchJSON, type DecisionDetail } from '../api/client'
import { formatTimeFull } from '../utils/time'

interface Props {
  projectId: string
  decisionId: string
  onClose: () => void
}

export default function DetailPanel({ projectId, decisionId, onClose }: Props) {
  const [detail, setDetail] = useState<DecisionDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [expandedChange, setExpandedChange] = useState<number | null>(null)

  useEffect(() => {
    setError('')
    setLoading(true)
    setExpandedChange(null)
    fetchJSON<DecisionDetail>(`/api/projects/${projectId}/decisions/${decisionId}`)
      .then(setDetail)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [projectId, decisionId])

  return (
    <div style={{
      width: '400px',
      borderLeft: '1px solid #333',
      background: '#1e1e1e',
      padding: '1rem',
      overflow: 'auto',
      flexShrink: 0,
      height: '100%',
    }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ margin: 0, fontSize: '0.85rem', color: '#888', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
          Decision Detail
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

      {loading && <div style={{ color: '#888', fontSize: '0.85rem' }}>Loading...</div>}
      {error && <div style={{ color: '#f66', fontSize: '0.85rem' }}>Error: {error}</div>}

      {detail && (
        <>
          {/* Title */}
          <h4 style={{ margin: '0 0 0.5rem', fontSize: '1.05rem', lineHeight: 1.3, color: '#e0e0e0' }}>
            {detail.title}
          </h4>

          {/* Meta */}
          <div style={{ fontSize: '0.8rem', color: '#888', marginBottom: '0.5rem', display: 'flex', gap: '6px' }}>
            <span>{detail.author}</span>
            <span style={{ color: '#555' }}>·</span>
            <span>{formatTimeFull(detail.instant)}</span>
          </div>

          {/* ID */}
          <div style={{ fontSize: '0.7rem', color: '#555', marginBottom: '1rem', fontFamily: 'monospace' }}>
            {detail.id.slice(0, 8)}
          </div>

          {/* Rationale */}
          {detail.rationale && (
            <InfoSection title="Rationale">
              <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc', lineHeight: 1.5 }}>
                {detail.rationale}
              </p>
            </InfoSection>
          )}

          {/* Context */}
          {detail.context && (
            <InfoSection title="Context">
              <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc', lineHeight: 1.5 }}>
                {detail.context}
              </p>
            </InfoSection>
          )}

          {/* Source Thread */}
          {detail.sourceThread && (
            <InfoSection title="Source Thread">
              <div style={{
                fontSize: '0.85rem',
                padding: '8px 10px',
                background: '#252525',
                borderRadius: '4px',
                borderLeft: '3px solid #22c55e',
              }}>
                <div style={{ color: '#e0e0e0' }}>{detail.sourceThread.title}</div>
                <div style={{ fontSize: '0.7rem', color: '#888', marginTop: '2px' }}>
                  <ThreadStatusBadge status={detail.sourceThread.status} />
                </div>
              </div>
            </InfoSection>
          )}

          {/* Changes */}
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
                      <div
                        onClick={() => setExpandedChange(isExpanded ? null : i)}
                        style={{
                          fontSize: '0.8rem',
                          padding: '6px 10px',
                          background: '#252525',
                          borderRadius: '4px',
                          borderLeft: `3px solid ${color}`,
                          cursor: 'pointer',
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                        }}
                        onMouseEnter={e => { e.currentTarget.style.background = '#2a2a2a' }}
                        onMouseLeave={e => { e.currentTarget.style.background = '#252525' }}
                      >
                        <span>
                          <span style={{ color: '#888' }}>{c.attribute.split('/').pop()}</span>
                        </span>
                        <span style={{ fontSize: '0.65rem', color }}>{label}</span>
                      </div>

                      {isExpanded && (isModify || isAdd || isDel) && (
                        <div style={{
                          fontSize: '0.75rem',
                          margin: '4px 0 0 0',
                          borderRadius: '4px',
                          overflow: 'hidden',
                          border: '1px solid #333',
                        }}>
                          {c.before !== null && (
                            <div style={{
                              padding: '6px 10px',
                              background: '#2a1515',
                              color: '#e88',
                              whiteSpace: 'pre-wrap',
                              wordBreak: 'break-word',
                              maxHeight: '200px',
                              overflow: 'auto',
                            }}>
                              <span style={{ color: '#f44', marginRight: '6px' }}>-</span>
                              {truncate(c.before, 500)}
                            </div>
                          )}
                          {c.after !== null && (
                            <div style={{
                              padding: '6px 10px',
                              background: '#152a15',
                              color: '#8e8',
                              whiteSpace: 'pre-wrap',
                              wordBreak: 'break-word',
                              maxHeight: '200px',
                              overflow: 'auto',
                            }}>
                              <span style={{ color: '#4c4', marginRight: '6px' }}>+</span>
                              {truncate(c.after, 500)}
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}

              {/* Non-content changes */}
              {detail.changes
                .filter(c => c.attribute !== 'section/content' && c.attribute !== 'section/title')
                .map((c, i) => (
                  <div key={`other-${i}`} style={{
                    fontSize: '0.75rem',
                    padding: '4px 10px',
                    color: '#666',
                  }}>
                    {c.entityType}/{c.attribute.split('/').pop()}
                  </div>
                ))}
            </InfoSection>
          )}

          {/* Related Tasks */}
          {detail.relatedTasks.length > 0 && (
            <InfoSection title="Related Tasks">
              {detail.relatedTasks.map(t => (
                <div key={t.id} style={{
                  fontSize: '0.85rem',
                  padding: '6px 10px',
                  margin: '4px 0',
                  background: '#252525',
                  borderRadius: '4px',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}>
                  <span style={{ color: '#ccc' }}>{t.title}</span>
                  <TaskStatusBadge status={t.status} />
                </div>
              ))}
            </InfoSection>
          )}
        </>
      )}
    </div>
  )
}

function InfoSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: '1rem' }}>
      <div style={{
        fontSize: '0.7rem',
        color: '#666',
        marginBottom: '6px',
        textTransform: 'uppercase',
        letterSpacing: '0.05em',
      }}>
        {title}
      </div>
      {children}
    </div>
  )
}

function TaskStatusBadge({ status }: { status: string }) {
  const colors: Record<string, { bg: string; fg: string }> = {
    todo: { bg: '#333', fg: '#aaa' },
    'in-progress': { bg: '#2a2a1a', fg: '#fa4' },
    done: { bg: '#1a2a1a', fg: '#4c4' },
    cancelled: { bg: '#2a1a1a', fg: '#f44' },
  }
  const c = colors[status] || colors.todo
  return (
    <span style={{
      fontSize: '0.65rem',
      padding: '1px 6px',
      borderRadius: '3px',
      background: c.bg,
      color: c.fg,
    }}>
      {status}
    </span>
  )
}

function ThreadStatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    open: '#22c55e',
    decided: '#16a34a',
    closed: '#888',
  }
  return (
    <span style={{
      fontSize: '0.65rem',
      padding: '1px 6px',
      borderRadius: '3px',
      background: '#252525',
      color: colors[status] || '#888',
    }}>
      {status}
    </span>
  )
}

function truncate(s: string, max: number): string {
  if (s.length <= max) return s
  return s.slice(0, max) + '...'
}
