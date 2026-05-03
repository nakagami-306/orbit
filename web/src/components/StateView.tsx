import { useEffect, useState } from 'react'
import { fetchJSON, type EntityNode, type SectionSummary } from '../api/client'

interface Props {
  projectId: string
  branch: string
  sections: EntityNode[]
}

export default function StateView({ projectId, branch, sections }: Props) {
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [details, setDetails] = useState<Record<string, SectionSummary>>({})
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    if (!projectId) return
    const branchParam = branch ? `?branch=${encodeURIComponent(branch)}` : ''
    fetchJSON<SectionSummary[]>(`/api/projects/${projectId}/sections${branchParam}`)
      .then(list => {
        const map: Record<string, SectionSummary> = {}
        for (const s of list) map[s.id] = s
        setDetails(map)
        setLoadError('')
      })
      .catch(e => setLoadError(e.message))
  }, [projectId, branch])

  // Sort: prefer position when content available, fall back to instant
  const sorted = [...sections].sort((a, b) => {
    const da = details[a.id]
    const db = details[b.id]
    if (da && db) return da.position - db.position
    return (a.instant || '').localeCompare(b.instant || '')
  })

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem' }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: '1rem',
        padding: '0 0 0.75rem',
        borderBottom: '1px solid #2a2a2a',
      }}>
        <div style={{ fontSize: '0.8rem', color: '#888' }}>
          {sorted.length} section{sorted.length !== 1 ? 's' : ''}
          {sorted.filter(s => s.status === 'stale').length > 0 && (
            <span style={{ marginLeft: '12px', color: '#eab308' }}>
              {sorted.filter(s => s.status === 'stale').length} stale
            </span>
          )}
        </div>
        {loadError && (
          <div style={{ fontSize: '0.75rem', color: '#f66' }}>
            content load failed: {loadError}
          </div>
        )}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {sorted.map((section, index) => {
          const isStale = section.status === 'stale'
          const isExpanded = expandedId === section.id
          const detail = details[section.id]

          return (
            <div
              key={section.id}
              style={{
                background: '#1e1e1e',
                borderRadius: '8px',
                border: isStale ? '1px solid #3a3520' : '1px solid #333',
                transition: 'border-color 0.15s',
              }}
            >
              <div
                onClick={() => setExpandedId(isExpanded ? null : section.id)}
                style={{
                  padding: '12px 14px',
                  cursor: 'pointer',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}
                onMouseEnter={e => { e.currentTarget.parentElement!.style.borderColor = '#555' }}
                onMouseLeave={e => {
                  e.currentTarget.parentElement!.style.borderColor = isStale ? '#3a3520' : '#333'
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <span style={{
                    fontSize: '0.75rem',
                    color: '#555',
                    fontFamily: 'monospace',
                    width: '24px',
                  }}>
                    {index + 1}.
                  </span>
                  <span style={{ fontSize: '0.9rem', fontWeight: 500, color: '#e0e0e0' }}>
                    {section.title}
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  {isStale ? (
                    <span style={{
                      fontSize: '0.65rem',
                      padding: '2px 6px',
                      borderRadius: '3px',
                      background: '#2a2510',
                      color: '#eab308',
                    }}>
                      stale
                    </span>
                  ) : (
                    <span style={{
                      fontSize: '0.65rem',
                      padding: '2px 6px',
                      borderRadius: '3px',
                      background: '#1a2a1a',
                      color: '#4c4',
                    }}>
                      current
                    </span>
                  )}

                  <span style={{
                    fontSize: '0.7rem',
                    color: '#555',
                    transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                    transition: 'transform 0.15s',
                  }}>
                    &#9660;
                  </span>
                </div>
              </div>

              {/* Expanded content */}
              {isExpanded && (
                <div style={{
                  padding: '12px 14px 14px',
                  borderTop: '1px solid #2a2a2a',
                }}>
                  {detail ? (
                    detail.content ? (
                      <div style={{
                        fontSize: '0.85rem',
                        color: '#d0d0d0',
                        whiteSpace: 'pre-wrap',
                        wordBreak: 'break-word',
                        lineHeight: 1.6,
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                        background: '#161616',
                        padding: '12px 14px',
                        borderRadius: '6px',
                        border: '1px solid #2a2a2a',
                      }}>
                        {detail.content}
                      </div>
                    ) : (
                      <div style={{ fontSize: '0.8rem', color: '#666', fontStyle: 'italic' }}>
                        (empty)
                      </div>
                    )
                  ) : (
                    <div style={{ fontSize: '0.8rem', color: '#666' }}>Loading content...</div>
                  )}

                  {detail?.isStale && detail.staleReason && (
                    <div style={{
                      marginTop: '10px',
                      fontSize: '0.75rem',
                      color: '#eab308',
                      padding: '6px 10px',
                      background: '#2a2510',
                      borderRadius: '4px',
                    }}>
                      stale: {detail.staleReason}
                    </div>
                  )}

                  <div style={{ marginTop: '10px', display: 'flex', gap: '14px', fontSize: '0.7rem', color: '#555' }}>
                    <span style={{ fontFamily: 'monospace' }}>{section.id}</span>
                    {section.instant && (
                      <span>updated {section.instant.slice(0, 19).replace('T', ' ')}</span>
                    )}
                  </div>
                </div>
              )}
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
          No sections yet. Create one with <code style={{ color: '#888' }}>orbit section add</code>.
        </div>
      )}
    </div>
  )
}
