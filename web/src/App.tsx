import { Routes, Route, Link } from 'react-router-dom'
import ProjectList from './pages/ProjectList'
import ProjectView from './pages/ProjectView'
import TaskBoard from './pages/TaskBoard'

export default function App() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <nav style={{
        display: 'flex',
        gap: '1rem',
        padding: '0.75rem 1.5rem',
        borderBottom: '1px solid #333',
        background: '#1a1a1a',
      }}>
        <Link to="/" style={{ color: '#fff', textDecoration: 'none', fontWeight: 'bold' }}>
          Orbit
        </Link>
        <Link to="/" style={{ color: '#aaa', textDecoration: 'none' }}>Projects</Link>
        <Link to="/tasks" style={{ color: '#aaa', textDecoration: 'none' }}>Tasks</Link>
      </nav>
      <main style={{ flex: 1, overflow: 'auto' }}>
        <Routes>
          <Route path="/" element={<ProjectList />} />
          <Route path="/projects/:id" element={<ProjectView />} />
          <Route path="/tasks" element={<TaskBoard />} />
        </Routes>
      </main>
    </div>
  )
}
