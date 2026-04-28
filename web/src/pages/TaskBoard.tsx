import { useEffect, useState } from 'react'
import { fetchJSON, patchJSON, type Task, type Commit } from '../api/client'

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
  const [expandedTask, setExpandedTask] = useState<string | null>(null)
  const [commits, setCommits] = useState<Record<string, Commit[]>>({})

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

  const toggleCommits = async (task: Task) => {
    if (expandedTask === task.id) {
      setExpandedTask(null)
      return
    }
    setExpandedTask(task.id)
    if (!commits[task.id]) {
      try {
        const data = await fetchJSON<Commit[]>(
          `/api/projects/${task.projectId}/commits?task=${task.id}`,
        )
        setCommits(prev => ({ ...prev, [task.id]: data }))
      } catch (e: any) {
        setError(e.message)
      }
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
                  const isExpanded = expandedTask === task.id
                  const taskCommits = commits[task.id] ?? []
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
                      <div style={{ fontSize: '0.7rem', color: '#888', display: 'flex', gap: '0.5rem', marginBottom: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
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
                        {task.gitBranch && (
                          <span title={`branch: ${task.gitBranch}`} style={{
                            color: '#aaa',
                            background: '#1a1a1a',
                            border: '1px solid #444',
                            padding: '0 5px',
                            borderRadius: '2px',
                            fontFamily: 'monospace',
                            fontSize: '0.65rem',
                          }}>
                            ⎇ {task.gitBranch}
                          </span>
                        )}
                        {task.commitCount > 0 && (
                          <button
                            onClick={() => toggleCommits(task)}
                            style={{
                              color: '#4caf50',
                              background: '#1a2a1a',
                              border: '1px solid #2d4a2d',
                              padding: '0 5px',
                              borderRadius: '2px',
                              fontFamily: 'monospace',
                              fontSize: '0.65rem',
                              cursor: 'pointer',
                            }}
                          >
                            {isExpanded ? '▾' : '▸'} {task.commitCount} commit{task.commitCount === 1 ? '' : 's'}
                          </button>
                        )}
                      </div>
                      {isExpanded && (
                        <div style={{
                          background: '#1a1a1a',
                          border: '1px solid #333',
                          borderRadius: '4px',
                          padding: '0.5rem',
                          marginBottom: '8px',
                          fontSize: '0.7rem',
                          maxHeight: '200px',
                          overflowY: 'auto',
                        }}>
                          {taskCommits.length === 0 ? (
                            <div style={{ color: '#666' }}>Loading…</div>
                          ) : (
                            taskCommits.map(c => (
                              <div key={c.id} style={{ display: 'flex', gap: '0.5rem', padding: '2px 0', borderBottom: '1px solid #2a2a2a' }}>
                                <span style={{ color: '#888', fontFamily: 'monospace' }}>{c.sha.slice(0, 7)}</span>
                                <span style={{ color: '#ddd', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                  {c.message}
                                </span>
                              </div>
                            ))
                          )}
                        </div>
                      )}
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
