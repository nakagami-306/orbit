import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'

interface ThreadNodeData {
  title: string
  status: string
  instant: string
  [key: string]: unknown
}

const statusColors: Record<string, { border: string; bg: string }> = {
  open: { border: '#22c55e', bg: '#0f2a1a' },
  decided: { border: '#16a34a', bg: '#1a2a1a' },
  closed: { border: '#555', bg: '#2a2a2a' },
}

function ThreadNode({ data, selected }: NodeProps) {
  const d = data as unknown as ThreadNodeData
  const style = statusColors[d.status] || statusColors.open
  const borderColor = selected ? '#fff' : style.border

  return (
    <div style={{
      padding: '6px 10px',
      background: style.bg,
      border: `1.5px solid ${borderColor}`,
      borderRadius: '4px',
      minWidth: '150px',
      maxWidth: '200px',
      cursor: 'pointer',
      transition: 'border-color 0.15s',
      boxShadow: selected ? '0 0 10px rgba(34, 197, 94, 0.3)' : 'none',
    }}>
      <Handle type="target" position={Position.Top} style={{ background: '#555', width: 5, height: 5 }} />

      <div style={{ fontSize: '0.55rem', color: '#22c55e', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '2px' }}>
        Thread
      </div>
      <div style={{ fontSize: '0.7rem', fontWeight: 600, lineHeight: 1.3, marginBottom: '2px' }}>
        {d.title}
      </div>
      <div style={{ fontSize: '0.55rem', color: '#888', display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
        <span style={{
          color: d.status === 'open' ? '#22c55e' : '#888',
        }}>{d.status}</span>
        {d.instant && <span>{d.instant.slice(0, 10)}</span>}
      </div>

      <Handle type="source" position={Position.Bottom} style={{ background: '#555', width: 5, height: 5 }} />
    </div>
  )
}

export default memo(ThreadNode)
