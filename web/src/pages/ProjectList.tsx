import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchJSON, type Project, type ProjectDetail } from '../api/client'

export default function ProjectList() {
  const [projects, setProjects] = useState<(Project & Partial<ProjectDetail>)[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    fetchJSON<Project[]>('/api/projects')
      .then(async (list) => {
        setProjects(list)
        // Fetch details in parallel for stats
        const detailed = await Promise.all(
          list.map(p =>
            fetchJSON<ProjectDetail>(`/api/projects/${p.id}`)
              .catch(() => p as Project & Partial<ProjectDetail>)
          )
        )
        setProjects(detailed)
      })
      .catch(e => setError(e.message))
  }, [])

  if (error) return <div style={{ padding: '2rem', color: '#f66' }}>Error: {error}</div>

  return (
    <div style={{ padding: '2rem', maxWidth: '1000px', margin: '0 auto' }}>
      <h1 style={{ marginBottom: '1.5rem' }}>Projects</h1>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '1rem' }}>
        {projects.map(p => (
          <Link
            key={p.id}
            to={`/projects/${p.id}`}
            style={{
              display: 'block',
              padding: '1.25rem',
              background: '#2a2a2a',
              borderRadius: '8px',
              textDecoration: 'none',
              color: 'inherit',
              border: '1px solid #333',
              transition: 'border-color 0.15s',
            }}
            onMouseEnter={e => (e.currentTarget.style.borderColor = '#555')}
            onMouseLeave={e => (e.currentTarget.style.borderColor = '#333')}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h2 style={{ margin: 0, fontSize: '1.2rem' }}>{p.name}</h2>
              <span style={{
                fontSize: '0.7rem',
                padding: '2px 8px',
                borderRadius: '4px',
                background: p.status === 'active' ? '#1a3a1a' : '#3a3a1a',
                color: p.status === 'active' ? '#4c4' : '#cc4',
              }}>
                {p.status}
              </span>
            </div>
            {p.description && (
              <p style={{ margin: '0.5rem 0 0', color: '#999', fontSize: '0.85rem', lineHeight: 1.4 }}>
                {p.description.length > 120 ? p.description.slice(0, 120) + '...' : p.description}
              </p>
            )}
            {p.decisions !== undefined && (
              <div style={{ marginTop: '0.75rem', fontSize: '0.75rem', color: '#666', display: 'flex', gap: '0.75rem' }}>
                <span>{p.decisions} decisions</span>
                <span>{p.sections} sections</span>
                {(p.pendingTasks ?? 0) > 0 && <span>{p.pendingTasks} tasks</span>}
                {(p.openThreads ?? 0) > 0 && <span style={{ color: '#fa4' }}>{p.openThreads} threads</span>}
              </div>
            )}
          </Link>
        ))}
      </div>
      {projects.length === 0 && (
        <div style={{ color: '#888', textAlign: 'center', marginTop: '4rem' }}>
          No projects found. Create one with <code>orbit init</code>.
        </div>
      )}
    </div>
  )
}
