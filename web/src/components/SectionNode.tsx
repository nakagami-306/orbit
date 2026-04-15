import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'

interface SectionNodeData {
  title: string
  status: string
  instant: string
  [key: string]: unknown
}

function SectionNode({ data, selected }: NodeProps) {
  const d = data as unknown as SectionNodeData
  const isStale = d.status === 'stale'
  const borderColor = selected ? '#fff' : isStale ? '#eab308' : '#555'
  const bgColor = isStale ? '#2a2510' : '#252525'

  return (
    <div style={{
      padding: '6px 10px',
      background: bgColor,
      border: `1.5px solid ${borderColor}`,
      borderRadius: '4px',
      minWidth: '150px',
      maxWidth: '200px',
      cursor: 'pointer',
      transition: 'border-color 0.15s',
      boxShadow: selected ? '0 0 10px rgba(255, 255, 255, 0.15)' : 'none',
    }}>
      <Handle type="target" position={Position.Top} style={{ background: '#555', width: 5, height: 5 }} />

      <div style={{ fontSize: '0.55rem', color: '#999', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '2px' }}>
        Section {isStale && <span style={{ color: '#eab308' }}>stale</span>}
      </div>
      <div style={{ fontSize: '0.7rem', fontWeight: 600, lineHeight: 1.3 }}>
        {d.title}
      </div>

      <Handle type="source" position={Position.Bottom} style={{ background: '#555', width: 5, height: 5 }} />
    </div>
  )
}

export default memo(SectionNode)
