import { useEffect, useState } from 'react'
import { fetchJSON, patchJSON, type Task } from '../api/client'

const columns = [
  { key: 'todo', label: 'Todo', color: '#888' },
  { key: 'in-progress', label: 'In Progress', color: '#fa4' },
  { key: 'done', label: 'Done', color: '#4c4' },
] as const

const priorityLabel: Record<string, { text: string; color: string }> = {
  h: { text: 'HIGH', color: '#f44' },
  m: { text: 'MED', color: '#fa4' },
  l: { text: 'LOW', color: '#4a4' },
  high: { text: 'HIGH', color: '#f44' },
  medium: { text: 'MED', color: '#fa4' },
  low: { text: 'LOW', color: '#4a4' },
}

export default function TaskBoard() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [error, setError] = useState('')
  const [hideCancelled, setHideCancelled] = useState(true)

  const loadTasks = () => {
    fetchJSON<Task[]>('/api/tasks')
      .then(setTasks)
      .catch(e => setError(e.message))
  }

  useEffect(() => { loadTasks() }, [])

  const moveTask = async (taskId: string, newStatus: string) => {
    // Optimistic update
    setTasks(prev => prev.map(t => t.id === taskId ? { ...t, status: newStatus } : t))
    try {
      await patchJSON(`/api/tasks/${taskId}`, { status: newStatus })
    } catch (e: any) {
      setError(e.message)
      loadTasks() // revert on failure
    }
  }

  const filteredTasks = hideCancelled ? tasks.filter(t => t.status !== 'cancelled') : tasks

  return (
    <div style={{ padding: '2rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <h1 style={{ margin: 0 }}>Task Board</h1>
        <label style={{ fontSize: '0.8rem', color: '#888', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '6px' }}>
          <input
            type="checkbox"
            checked={hideCancelled}
            onChange={e => setHideCancelled(e.target.checked)}
          />
          Hide cancelled
        </label>
      </div>

      {error && <div style={{ color: '#f66', marginBottom: '1rem' }}>Error: {error}</div>}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '1rem', minHeight: '60vh' }}>
        {columns.map(col => {
          const colTasks = filteredTasks.filter(t => t.status === col.key)
          return (
            <div key={col.key} style={{
              background: '#1e1e1e',
              borderRadius: '8px',
              padding: '1rem',
              border: '1px solid #333',
            }}>
              <h3 style={{
                margin: '0 0 1rem',
                fontSize: '0.85rem',
                display: 'flex',
                alignItems: 'center',
                gap: '0.5rem',
              }}>
                <span style={{
                  width: '8px', height: '8px', borderRadius: '50%',
                  background: col.color, display: 'inline-block',
                }} />
                {col.label}
                <span style={{ color: '#555', fontWeight: 400 }}>({colTasks.length})</span>
              </h3>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                {colTasks.map(task => {
                  const prio = priorityLabel[task.priority]
                  return (
                    <div key={task.id} style={{
                      background: '#2a2a2a',
                      borderRadius: '6px',
                      padding: '0.75rem',
                      border: '1px solid #333',
                    }}>
                      <div style={{ fontSize: '0.85rem', fontWeight: 500, marginBottom: '6px', lineHeight: 1.3 }}>
                        {task.title}
                      </div>
                      <div style={{ fontSize: '0.7rem', color: '#888', display: 'flex', gap: '0.5rem', marginBottom: '8px', flexWrap: 'wrap' }}>
                        {prio && (
                          <span style={{
                            color: prio.color,
                            background: `${prio.color}15`,
                            padding: '0 4px',
                            borderRadius: '2px',
                            fontSize: '0.6rem',
                            fontWeight: 600,
                          }}>
                            {prio.text}
                          </span>
                        )}
                        <span style={{ color: '#4a9eff' }}>{task.projectName}</span>
                        {task.assignee && <span>@{task.assignee}</span>}
                      </div>
                      <div style={{ display: 'flex', gap: '4px' }}>
                        {columns.filter(c => c.key !== col.key).map(c => (
                          <button
                            key={c.key}
                            onClick={() => moveTask(task.id, c.key)}
                            style={{
                              fontSize: '0.65rem',
                              padding: '3px 8px',
                              background: '#333',
                              color: '#aaa',
                              border: '1px solid #444',
                              borderRadius: '3px',
                              cursor: 'pointer',
                            }}
                            onMouseEnter={e => { e.currentTarget.style.background = '#444' }}
                            onMouseLeave={e => { e.currentTarget.style.background = '#333' }}
                          >
                            → {c.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
