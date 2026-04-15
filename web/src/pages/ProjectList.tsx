import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { fetchJSON, type Project } from '../api/client'

export default function ProjectList() {
  const [projects, setProjects] = useState<Project[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    fetchJSON<Project[]>('/api/projects')
      .then(setProjects)
      .catch(e => setError(e.message))
  }, [])

  if (error) return <div style={{ padding: '2rem', color: '#f66' }}>Error: {error}</div>

  return (
    <div style={{ padding: '2rem' }}>
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
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h2 style={{ margin: 0, fontSize: '1.2rem' }}>{p.name}</h2>
              <span style={{
                fontSize: '0.75rem',
                padding: '2px 8px',
                borderRadius: '4px',
                background: p.status === 'active' ? '#1a3a1a' : '#3a3a1a',
                color: p.status === 'active' ? '#4c4' : '#cc4',
              }}>
                {p.status}
              </span>
            </div>
            {p.description && (
              <p style={{ margin: '0.5rem 0 0', color: '#999', fontSize: '0.9rem' }}>
                {p.description.length > 120 ? p.description.slice(0, 120) + '...' : p.description}
              </p>
            )}
          </Link>
        ))}
      </div>
    </div>
  )
}
