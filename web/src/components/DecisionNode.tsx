import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'

interface DecisionNodeData {
  title: string
  author: string
  instant: string
  type: 'root' | 'normal' | 'merge'
  rationale: string
  milestone: string | null
  sourceThreadId: string | null
  [key: string]: unknown
}

const typeStyles: Record<string, { border: string; bg: string }> = {
  root: { border: '#4a9eff', bg: '#1a2a3a' },
  normal: { border: '#555', bg: '#2a2a2a' },
  merge: { border: '#a855f7', bg: '#2a1a3a' },
}

function DecisionNode({ data, selected }: NodeProps) {
  const d = data as unknown as DecisionNodeData
  const style = typeStyles[d.type] || typeStyles.normal
  const borderColor = selected ? '#fff' : style.border

  return (
    <div style={{
      padding: '8px 12px',
      background: style.bg,
      border: `2px solid ${borderColor}`,
      borderRadius: '6px',
      minWidth: '220px',
      maxWidth: '280px',
      cursor: 'pointer',
      transition: 'border-color 0.15s',
      boxShadow: selected ? '0 0 12px rgba(74, 158, 255, 0.3)' : 'none',
    }}>
      <Handle type="target" position={Position.Top} style={{ background: '#555', width: 6, height: 6 }} />

      {/* Milestone badge */}
      {d.milestone && (
        <div style={{
          fontSize: '0.6rem',
          color: '#f0c040',
          background: '#3a3520',
          padding: '1px 6px',
          borderRadius: '3px',
          marginBottom: '4px',
          display: 'inline-block',
        }}>
          ◆ {d.milestone}
        </div>
      )}

      <div style={{ fontSize: '0.8rem', fontWeight: 600, marginBottom: '4px', lineHeight: 1.3 }}>
        {d.title}
      </div>

      <div style={{ fontSize: '0.65rem', color: '#888', display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
        <span>{d.author}</span>
        <span>{d.instant?.slice(0, 10)}</span>
        {d.type === 'merge' && <span style={{ color: '#a855f7' }}>merge</span>}
        {d.sourceThreadId && <span style={{ color: '#4a9' }}>thread</span>}
      </div>

      <Handle type="source" position={Position.Bottom} style={{ background: '#555', width: 6, height: 6 }} />
    </div>
  )
}

export default memo(DecisionNode)
