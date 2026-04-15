import { useEffect, useState } from 'react'
import { fetchJSON, patchJSON, type Task } from '../api/client'

const columns = [
  { key: 'todo', label: 'Todo' },
  { key: 'in-progress', label: 'In Progress' },
  { key: 'done', label: 'Done' },
] as const

const priorityColor: Record<string, string> = {
  h: '#f44',
  m: '#fa4',
  l: '#4a4',
  high: '#f44',
  medium: '#fa4',
  low: '#4a4',
}

export default function TaskBoard() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [error, setError] = useState('')

  const loadTasks = () => {
    fetchJSON<Task[]>('/api/tasks')
      .then(setTasks)
      .catch(e => setError(e.message))
  }

  useEffect(() => { loadTasks() }, [])

  const moveTask = async (taskId: string, newStatus: string) => {
    try {
      await patchJSON(`/api/tasks/${taskId}`, { status: newStatus })
      loadTasks()
    } catch (e: any) {
      setError(e.message)
    }
  }

  if (error) return <div style={{ padding: '2rem', color: '#f66' }}>Error: {error}</div>

  return (
    <div style={{ padding: '2rem' }}>
      <h1 style={{ marginBottom: '1.5rem' }}>Task Board</h1>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '1rem', minHeight: '60vh' }}>
        {columns.map(col => {
          const colTasks = tasks.filter(t => t.status === col.key)
          return (
            <div key={col.key} style={{
              background: '#222',
              borderRadius: '8px',
              padding: '1rem',
              border: '1px solid #333',
            }}>
              <h3 style={{ margin: '0 0 1rem', color: '#aaa', fontSize: '0.9rem' }}>
                {col.label} ({colTasks.length})
              </h3>
              {colTasks.map(task => (
                <div key={task.id} style={{
                  background: '#2a2a2a',
                  borderRadius: '6px',
                  padding: '0.75rem',
                  marginBottom: '0.5rem',
                  border: '1px solid #333',
                }}>
                  <div style={{ fontSize: '0.85rem', fontWeight: 500, marginBottom: '4px' }}>
                    {task.title}
                  </div>
                  <div style={{ fontSize: '0.7rem', color: '#888', display: 'flex', gap: '0.5rem', marginBottom: '6px' }}>
                    <span style={{ color: priorityColor[task.priority] || '#888' }}>
                      {task.priority}
                    </span>
                    <span>{task.projectName}</span>
                  </div>
                  <div style={{ display: 'flex', gap: '4px' }}>
                    {columns.filter(c => c.key !== col.key).map(c => (
                      <button
                        key={c.key}
                        onClick={() => moveTask(task.id, c.key)}
                        style={{
                          fontSize: '0.65rem',
                          padding: '2px 6px',
                          background: '#333',
                          color: '#aaa',
                          border: '1px solid #444',
                          borderRadius: '3px',
                          cursor: 'pointer',
                        }}
                      >
                        → {c.label}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )
        })}
      </div>
    </div>
  )
}
