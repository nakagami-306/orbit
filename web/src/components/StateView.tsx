import { useState } from 'react'
import type { EntityNode } from '../api/client'

interface Props {
  sections: EntityNode[]
}

export default function StateView({ sections }: Props) {
  const [expandedId, setExpandedId] = useState<string | null>(null)

  // Sort sections by instant (earliest = first position)
  const sorted = [...sections].sort((a, b) =>
    (a.instant || '').localeCompare(b.instant || '')
  )

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
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {sorted.map((section, index) => {
          const isStale = section.status === 'stale'
          const isExpanded = expandedId === section.id

          return (
            <div
              key={section.id}
              onClick={() => setExpandedId(isExpanded ? null : section.id)}
              style={{
                padding: '12px 14px',
                background: '#1e1e1e',
                borderRadius: '8px',
                border: isStale ? '1px solid #3a3520' : '1px solid #333',
                cursor: 'pointer',
                transition: 'border-color 0.15s',
              }}
              onMouseEnter={e => { e.currentTarget.style.borderColor = '#555' }}
              onMouseLeave={e => {
                e.currentTarget.style.borderColor = isStale ? '#3a3520' : '#333'
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
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
                  marginTop: '10px',
                  paddingTop: '10px',
                  borderTop: '1px solid #2a2a2a',
                }}>
                  <div style={{ fontSize: '0.7rem', color: '#555', marginBottom: '4px', fontFamily: 'monospace' }}>
                    {section.id}
                  </div>
                  {section.instant && (
                    <div style={{ fontSize: '0.75rem', color: '#888' }}>
                      Last updated: {section.instant.slice(0, 19).replace('T', ' ')}
                    </div>
                  )}
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
