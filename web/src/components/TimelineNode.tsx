import { memo } from 'react'
import type { DAGNode, EntityNode } from '../api/client'
import { formatTimeShort } from '../utils/time'

export interface TimelineNodeProps {
  decision: DAGNode
  isSelected: boolean
  onClick: () => void
  relatedThreads: EntityNode[]
  relatedTasks: EntityNode[]
  relatedSections: EntityNode[]
  milestone: string | null
  sourceThread: EntityNode | null
  parentCount: number
  isRoot: boolean
}

function TimelineNode({
  decision,
  isSelected,
  onClick,
  relatedThreads,
  relatedTasks,
  relatedSections,
  milestone,
  sourceThread,
  parentCount,
  isRoot,
}: TimelineNodeProps) {
  const dotColor = isRoot
    ? '#4a9eff'
    : decision.type === 'merge'
      ? '#a855f7'
      : '#666'

  const sectionCount = relatedSections.length
  const taskCount = relatedTasks.length
  const threadCount = relatedThreads.length

  const summaryParts: string[] = []
  if (sectionCount > 0) summaryParts.push(`${sectionCount} section${sectionCount !== 1 ? 's' : ''}`)
  if (taskCount > 0) summaryParts.push(`${taskCount} task${taskCount !== 1 ? 's' : ''}`)
  if (threadCount > 0 && !sourceThread) summaryParts.push(`${threadCount} thread${threadCount !== 1 ? 's' : ''}`)

  return (
    <div
      onClick={onClick}
      style={{
        display: 'flex',
        gap: '0',
        cursor: 'pointer',
        position: 'relative',
      }}
    >
      {/* Timeline track */}
      <div style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        width: '40px',
        flexShrink: 0,
        position: 'relative',
      }}>
        {/* Line above dot */}
        <div style={{
          width: '2px',
          flex: 1,
          background: '#333',
          minHeight: '12px',
        }} />
        {/* Dot */}
        <div style={{
          width: isRoot ? '14px' : '10px',
          height: isRoot ? '14px' : '10px',
          borderRadius: '50%',
          background: dotColor,
          border: isSelected ? '2px solid #fff' : `2px solid ${dotColor}`,
          flexShrink: 0,
          zIndex: 1,
          boxShadow: isSelected ? `0 0 8px ${dotColor}` : 'none',
        }} />
        {/* Line below dot */}
        <div style={{
          width: '2px',
          flex: 1,
          background: '#333',
          minHeight: '12px',
        }} />
      </div>

      {/* Card */}
      <div style={{
        flex: 1,
        padding: '10px 14px',
        margin: '4px 0',
        background: isSelected ? '#1e2a3a' : '#1e1e1e',
        borderRadius: '8px',
        border: isSelected ? '1px solid #4a9eff' : '1px solid #333',
        borderLeft: isSelected ? '3px solid #4a9eff' : '3px solid transparent',
        transition: 'all 0.15s',
      }}
        onMouseEnter={e => {
          if (!isSelected) {
            e.currentTarget.style.borderColor = '#555'
            e.currentTarget.style.background = '#222'
          }
        }}
        onMouseLeave={e => {
          if (!isSelected) {
            e.currentTarget.style.borderColor = '#333'
            e.currentTarget.style.background = '#1e1e1e'
          }
        }}
      >
        {/* Title row */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '1rem' }}>
          <div style={{ fontSize: '0.9rem', fontWeight: 600, lineHeight: 1.3, color: '#e0e0e0' }}>
            {decision.title}
          </div>
          <div style={{
            fontSize: '0.7rem',
            color: '#888',
            whiteSpace: 'nowrap',
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}>
            <span>{decision.author}</span>
            <span style={{ color: '#555' }}>·</span>
            <span>{formatTimeShort(decision.instant)}</span>
          </div>
        </div>

        {/* Type / branch badges */}
        <div style={{ display: 'flex', gap: '6px', marginTop: '4px', flexWrap: 'wrap' }}>
          {decision.type === 'merge' && (
            <span style={{
              fontSize: '0.65rem',
              padding: '1px 6px',
              borderRadius: '3px',
              background: '#2a1a3a',
              color: '#a855f7',
            }}>
              merge
            </span>
          )}
          {isRoot && (
            <span style={{
              fontSize: '0.65rem',
              padding: '1px 6px',
              borderRadius: '3px',
              background: '#1a2a3a',
              color: '#4a9eff',
            }}>
              root
            </span>
          )}
          {parentCount > 1 && (
            <span style={{
              fontSize: '0.65rem',
              padding: '1px 6px',
              borderRadius: '3px',
              background: '#2a2a2a',
              color: '#888',
            }}>
              {parentCount} parents
            </span>
          )}
        </div>

        {/* Related entities */}
        <div style={{ marginTop: '6px', display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {sourceThread && (
            <div style={{ fontSize: '0.75rem', color: '#22c55e', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <span style={{ color: '#555' }}>&larr;</span>
              <span style={{ color: '#22c55e' }}>{sourceThread.title}</span>
              <span style={{
                fontSize: '0.6rem',
                padding: '0 4px',
                borderRadius: '2px',
                background: '#0f2a1a',
                color: '#22c55e',
              }}>thread</span>
            </div>
          )}
          {summaryParts.length > 0 && (
            <div style={{ fontSize: '0.75rem', color: '#888', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <span style={{ color: '#555' }}>&rarr;</span>
              <span>{summaryParts.join(' · ')}</span>
            </div>
          )}
          {milestone && (
            <div style={{ fontSize: '0.75rem', color: '#f0c040', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <span style={{ color: '#f0c040' }}>&#9670;</span>
              <span>{milestone}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default memo(TimelineNode)
