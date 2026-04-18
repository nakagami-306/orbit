import { memo } from 'react'
import type { ProcessedDecision } from './Timeline'
import { formatTimeShort } from '../utils/time'

interface Props {
  item: ProcessedDecision
  isSelected: boolean
  onClick: () => void
  isFirst: boolean
  isLast: boolean
}

function TimelineNode({ item, isSelected, onClick, isFirst, isLast }: Props) {
  const {
    decision, relatedThreads, relatedTasks, relatedSections,
    milestone, sourceThread, parentCount, isRoot, parentTitles, childCount,
  } = item

  const dotColor = isRoot
    ? '#4a9eff'
    : decision.type === 'merge'
      ? '#a855f7'
      : '#555'

  const dotSize = isRoot ? 14 : 10
  const hasEntities = relatedSections.length > 0 || relatedTasks.length > 0
    || relatedThreads.length > 0 || sourceThread

  return (
    <div style={{ position: 'relative', paddingBottom: isLast ? 0 : '4px' }}>
      {/* Dot on the timeline line */}
      <div style={{
        position: 'absolute',
        left: -dotSize / 2,
        top: '18px',
        width: `${dotSize}px`,
        height: `${dotSize}px`,
        borderRadius: '50%',
        background: dotColor,
        border: isSelected ? '2px solid #e0e0e0' : 'none',
        boxShadow: isSelected ? `0 0 10px ${dotColor}` : 'none',
        zIndex: 2,
      }} />

      {/* Card */}
      <div
        onClick={onClick}
        style={{
          marginLeft: '20px',
          cursor: 'pointer',
          borderRadius: '8px',
          border: isSelected ? '1px solid #4a9eff' : '1px solid transparent',
          background: isSelected ? '#141c28' : 'transparent',
          transition: 'all 0.15s',
          overflow: 'hidden',
        }}
        onMouseEnter={e => {
          if (!isSelected) e.currentTarget.style.background = '#181818'
        }}
        onMouseLeave={e => {
          if (!isSelected) e.currentTarget.style.background = 'transparent'
        }}
      >
        {/* Decision header */}
        <div style={{ padding: '10px 14px 6px' }}>
          {/* Parent link - shows the DAG connection */}
          {!isRoot && parentTitles.length > 0 && (
            <div style={{
              fontSize: '0.65rem',
              color: '#555',
              marginBottom: '4px',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
            }}>
              <span style={{ color: '#3a3a3a' }}>&#9585;</span>
              {parentCount > 1
                ? <span>{parentCount} parents (merge)</span>
                : <span>follows: {truncate(parentTitles[0], 50)}</span>
              }
            </div>
          )}

          {/* Title row */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '1rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
              <span style={{ fontSize: '0.9rem', fontWeight: 600, lineHeight: 1.3, color: '#e0e0e0' }}>
                {decision.title}
              </span>
              {isRoot && (
                <span style={{
                  fontSize: '0.6rem', padding: '1px 5px', borderRadius: '3px',
                  background: '#1a2a3a', color: '#4a9eff',
                }}>root</span>
              )}
              {decision.type === 'merge' && (
                <span style={{
                  fontSize: '0.6rem', padding: '1px 5px', borderRadius: '3px',
                  background: '#2a1a3a', color: '#a855f7',
                }}>merge</span>
              )}
            </div>
            <div style={{
              fontSize: '0.7rem', color: '#666', whiteSpace: 'nowrap', flexShrink: 0,
            }}>
              {decision.author} · {formatTimeShort(decision.instant)}
            </div>
          </div>

          {/* Milestone */}
          {milestone && (
            <div style={{
              marginTop: '4px', fontSize: '0.7rem',
              display: 'inline-flex', alignItems: 'center', gap: '4px',
              padding: '2px 8px', borderRadius: '3px',
              background: '#2a2510', color: '#f0c040',
            }}>
              &#9670; {milestone}
            </div>
          )}

          {/* Child count hint */}
          {childCount > 1 && (
            <div style={{
              fontSize: '0.65rem', color: '#444', marginTop: '2px',
            }}>
              &#9581; {childCount} follow-up decisions
            </div>
          )}
        </div>

        {/* Related entities - only if any exist */}
        {hasEntities && (
          <div style={{
            padding: '6px 14px 10px',
            display: 'flex',
            flexDirection: 'column',
            gap: '4px',
          }}>
            {/* Source thread - the input that led to this decision */}
            {sourceThread && (
              <EntityChip
                color="#22c55e"
                bg="#0a1f14"
                border="#163b28"
                label="THREAD"
                title={sourceThread.title}
                status={sourceThread.status}
                direction="in"
              />
            )}

            {/* Other threads (not the source) */}
            {relatedThreads.filter(t => t.id !== sourceThread?.id).map(t => (
              <EntityChip
                key={t.id}
                color="#22c55e"
                bg="#0a1f14"
                border="#163b28"
                label="THREAD"
                title={t.title}
                status={t.status}
              />
            ))}

            {/* Sections affected */}
            {relatedSections.map(s => (
              <EntityChip
                key={s.id}
                color={s.status === 'stale' ? '#eab308' : '#8b8b8b'}
                bg={s.status === 'stale' ? '#1a1a0a' : '#141414'}
                border={s.status === 'stale' ? '#2a2a10' : '#252525'}
                label="SECTION"
                title={s.title}
                status={s.status === 'stale' ? 'stale' : undefined}
                direction="out"
              />
            ))}

            {/* Tasks spawned */}
            {relatedTasks.map(t => (
              <EntityChip
                key={t.id}
                color="#f97316"
                bg="#1a1208"
                border="#2a2010"
                label="TASK"
                title={t.title}
                status={t.status.split(' ')[0]}
                direction="out"
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function EntityChip({ color, bg, border, label, title, status, direction }: {
  color: string
  bg: string
  border: string
  label: string
  title: string
  status?: string
  direction?: 'in' | 'out'
}) {
  const arrow = direction === 'in' ? '\u2190' : direction === 'out' ? '\u2192' : ''

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      gap: '6px',
      padding: '3px 8px',
      background: bg,
      border: `1px solid ${border}`,
      borderRadius: '4px',
      fontSize: '0.75rem',
    }}>
      {arrow && <span style={{ color: '#444', fontSize: '0.7rem' }}>{arrow}</span>}
      <span style={{
        fontSize: '0.55rem',
        fontWeight: 700,
        color,
        letterSpacing: '0.05em',
        flexShrink: 0,
        minWidth: '52px',
      }}>
        {label}
      </span>
      <span style={{ color: '#bbb', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {title}
      </span>
      {status && (
        <span style={{
          fontSize: '0.6rem',
          padding: '0 4px',
          borderRadius: '2px',
          color,
          opacity: 0.8,
          flexShrink: 0,
        }}>
          {status}
        </span>
      )}
    </div>
  )
}

function truncate(s: string, max: number): string {
  return s.length <= max ? s : s.slice(0, max) + '...'
}

export default memo(TimelineNode)
