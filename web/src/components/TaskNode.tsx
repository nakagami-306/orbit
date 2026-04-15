import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { formatTimeShort } from '../utils/time'

interface TaskNodeData {
  title: string
  status: string
  instant: string
  [key: string]: unknown
}

const statusColors: Record<string, { border: string; bg: string }> = {
  todo: { border: '#f97316', bg: '#2a1a0a' },
  'in-progress': { border: '#fb923c', bg: '#2a200a' },
  done: { border: '#555', bg: '#2a2a2a' },
  cancelled: { border: '#555', bg: '#2a2a2a' },
}

function TaskNode({ data, selected }: NodeProps) {
  const d = data as unknown as TaskNodeData
  // status might include priority like "todo (high)", extract base status
  const baseStatus = d.status.split(' ')[0]
  const style = statusColors[baseStatus] || statusColors.todo
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
      boxShadow: selected ? '0 0 10px rgba(249, 115, 22, 0.3)' : 'none',
    }}>
      <Handle type="target" position={Position.Top} style={{ background: '#555', width: 5, height: 5 }} />

      <div style={{ fontSize: '0.55rem', color: '#f97316', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '2px' }}>
        Task
      </div>
      <div style={{ fontSize: '0.7rem', fontWeight: 600, lineHeight: 1.3, marginBottom: '2px' }}>
        {d.title}
      </div>
      <div style={{ fontSize: '0.55rem', color: '#888', display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
        <span style={{
          color: baseStatus === 'todo' || baseStatus === 'in-progress' ? '#f97316' : '#888',
        }}>{d.status}</span>
        {d.instant && <span>{formatTimeShort(d.instant)}</span>}
      </div>

      <Handle type="source" position={Position.Bottom} style={{ background: '#555', width: 5, height: 5 }} />
    </div>
  )
}

export default memo(TaskNode)
