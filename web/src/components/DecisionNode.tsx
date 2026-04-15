import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'

interface DecisionNodeData {
  title: string
  author: string
  instant: string
  type: 'root' | 'normal' | 'merge'
  rationale: string
  [key: string]: unknown
}

const typeColors: Record<string, string> = {
  root: '#4a9eff',
  normal: '#666',
  merge: '#a855f7',
}

function DecisionNode({ data, selected }: NodeProps) {
  const d = data as unknown as DecisionNodeData
  const borderColor = selected ? '#fff' : typeColors[d.type] || '#666'

  return (
    <div style={{
      padding: '8px 12px',
      background: '#2a2a2a',
      border: `2px solid ${borderColor}`,
      borderRadius: '6px',
      minWidth: '200px',
      maxWidth: '280px',
      cursor: 'pointer',
    }}>
      <Handle type="target" position={Position.Top} style={{ background: '#555' }} />
      <div style={{ fontSize: '0.8rem', fontWeight: 600, marginBottom: '4px' }}>
        {d.title}
      </div>
      <div style={{ fontSize: '0.65rem', color: '#888', display: 'flex', gap: '0.5rem' }}>
        <span>{d.author}</span>
        <span>{d.instant?.slice(0, 10)}</span>
      </div>
      <Handle type="source" position={Position.Bottom} style={{ background: '#555' }} />
    </div>
  )
}

export default memo(DecisionNode)
