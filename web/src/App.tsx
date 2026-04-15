import { Routes, Route, Link, useLocation } from 'react-router-dom'
import ProjectList from './pages/ProjectList'
import ProjectView from './pages/ProjectView'
import TaskBoard from './pages/TaskBoard'

function NavLink({ to, children }: { to: string; children: React.ReactNode }) {
  const location = useLocation()
  const isActive = to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
  return (
    <Link to={to} style={{
      color: isActive ? '#e0e0e0' : '#888',
      textDecoration: 'none',
      fontSize: '0.9rem',
      padding: '4px 0',
      borderBottom: isActive ? '2px solid #4a9eff' : '2px solid transparent',
    }}>
      {children}
    </Link>
  )
}

export default function App() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <nav style={{
        display: 'flex',
        alignItems: 'center',
        gap: '1.5rem',
        padding: '0 1.5rem',
        height: '48px',
        borderBottom: '1px solid #2a2a2a',
        background: '#161616',
        flexShrink: 0,
      }}>
        <Link to="/" style={{ color: '#fff', textDecoration: 'none', fontWeight: 700, fontSize: '1rem', marginRight: '0.5rem' }}>
          Orbit
        </Link>
        <NavLink to="/">Projects</NavLink>
        <NavLink to="/tasks">Tasks</NavLink>
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
