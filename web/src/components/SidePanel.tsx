import { useEffect, useState } from 'react'
import { fetchJSON, type DecisionDetail } from '../api/client'

interface Props {
  projectId: string
  decisionId: string | null
  onClose: () => void
}

export default function SidePanel({ projectId, decisionId, onClose }: Props) {
  const [detail, setDetail] = useState<DecisionDetail | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!decisionId) { setDetail(null); return }
    setError('')
    fetchJSON<DecisionDetail>(`/api/projects/${projectId}/decisions/${decisionId}`)
      .then(setDetail)
      .catch(e => setError(e.message))
  }, [projectId, decisionId])

  if (!decisionId) return null

  return (
    <div style={{
      width: '400px',
      borderLeft: '1px solid #333',
      background: '#1e1e1e',
      padding: '1rem',
      overflow: 'auto',
      flexShrink: 0,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ margin: 0 }}>Decision Detail</h3>
        <button onClick={onClose} style={{
          background: 'none', border: 'none', color: '#888', cursor: 'pointer', fontSize: '1.2rem',
        }}>x</button>
      </div>

      {error && <div style={{ color: '#f66' }}>Error: {error}</div>}

      {detail && (
        <>
          <h4 style={{ margin: '0 0 0.5rem' }}>{detail.title}</h4>
          <div style={{ fontSize: '0.8rem', color: '#888', marginBottom: '1rem' }}>
            {detail.author} &middot; {detail.instant?.slice(0, 19).replace('T', ' ')}
          </div>

          {detail.rationale && (
            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontSize: '0.75rem', color: '#666', marginBottom: '4px' }}>Rationale</div>
              <p style={{ margin: 0, fontSize: '0.85rem', color: '#ccc' }}>{detail.rationale}</p>
            </div>
          )}

          {detail.sourceThread && (
            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontSize: '0.75rem', color: '#666', marginBottom: '4px' }}>Source Thread</div>
              <p style={{ margin: 0, fontSize: '0.85rem' }}>
                {detail.sourceThread.title} ({detail.sourceThread.status})
              </p>
            </div>
          )}

          {detail.changes.length > 0 && (
            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontSize: '0.75rem', color: '#666', marginBottom: '4px' }}>
                Changes ({detail.changes.length})
              </div>
              {detail.changes.map((c, i) => (
                <div key={i} style={{
                  fontSize: '0.8rem',
                  padding: '4px 8px',
                  margin: '4px 0',
                  background: '#252525',
                  borderRadius: '4px',
                  borderLeft: `3px solid ${c.before === null ? '#4c4' : c.after === null ? '#f44' : '#fa4'}`,
                }}>
                  <span style={{ color: '#888' }}>{c.entityType}/</span>
                  {c.attribute.split('/').pop()}
                </div>
              ))}
            </div>
          )}

          {detail.relatedTasks.length > 0 && (
            <div>
              <div style={{ fontSize: '0.75rem', color: '#666', marginBottom: '4px' }}>Related Tasks</div>
              {detail.relatedTasks.map(t => (
                <div key={t.id} style={{ fontSize: '0.85rem', margin: '4px 0' }}>
                  [{t.status}] {t.title}
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
