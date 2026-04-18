import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { fetchJSON, type DAGResponse, type ProjectDetail, type EntityNode, type BranchInfo } from '../api/client'
import Timeline from '../components/Timeline'
import DetailPanel, { type PanelTarget } from '../components/DetailPanel'
import StateView from '../components/StateView'
import ThreadList from '../components/ThreadList'

type Tab = 'timeline' | 'state' | 'threads' | 'tasks'

export default function ProjectView() {
  const { id } = useParams<{ id: string }>()
  const [project, setProject] = useState<ProjectDetail | null>(null)
  const [dag, setDag] = useState<DAGResponse | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('timeline')
  const [panelTarget, setPanelTarget] = useState<PanelTarget | null>(null)
  const [selectedBranch, setSelectedBranch] = useState<string>('')
  const [error, setError] = useState('')

  const loadDAG = useCallback((branchName?: string) => {
    if (!id) return
    const branchParam = branchName ? `?branch=${encodeURIComponent(branchName)}` : ''
    fetchJSON<DAGResponse>(`/api/projects/${id}/dag${branchParam}`)
      .then(setDag)
      .catch(e => setError(e.message))
  }, [id])

  useEffect(() => {
    if (!id) return
    fetchJSON<ProjectDetail>(`/api/projects/${id}`)
      .then(setProject)
      .catch(e => setError(e.message))
    loadDAG()
  }, [id, loadDAG])

  // Close panel when switching tabs
  useEffect(() => { setPanelTarget(null) }, [activeTab])

  const selectDecision = useCallback((decisionId: string | null) => {
    setPanelTarget(decisionId ? { kind: 'decision', id: decisionId } : null)
  }, [])

  const selectThread = useCallback((threadId: string) => {
    setPanelTarget({ kind: 'thread', id: threadId })
  }, [])

  const handleBranchChange = useCallback((branch: string) => {
    setSelectedBranch(branch)
    loadDAG(branch || undefined)
  }, [loadDAG])

  const branches = dag?.branches || []
  const selectedDecisionId = panelTarget?.kind === 'decision' ? panelTarget.id : null

  const tabs: { key: Tab; label: string; count?: number }[] = [
    { key: 'timeline', label: 'Timeline', count: dag?.nodes.length },
    { key: 'state', label: 'State', count: dag?.sections?.length },
    { key: 'threads', label: 'Threads', count: dag?.threads?.length },
    { key: 'tasks', label: 'Tasks', count: dag?.tasks?.length },
  ]

  if (error) return <div style={{ padding: '2rem', color: '#f66' }}>Error: {error}</div>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Project header */}
      <div style={{
        padding: '1rem 1.5rem',
        borderBottom: '1px solid #2a2a2a',
        background: '#161616',
        flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '8px' }}>
          <Link to="/" style={{ color: '#888', textDecoration: 'none', fontSize: '0.8rem' }}>Projects</Link>
          <span style={{ color: '#555' }}>/</span>
          <h2 style={{ margin: 0, fontSize: '1.1rem', color: '#e0e0e0' }}>{project?.name || '...'}</h2>

          {/* Branch selector */}
          {branches.length > 1 && (
            <select
              value={selectedBranch}
              onChange={e => handleBranchChange(e.target.value)}
              style={{
                marginLeft: '12px',
                background: '#2a2a2a',
                border: '1px solid #444',
                color: '#ccc',
                fontSize: '0.75rem',
                padding: '2px 6px',
                borderRadius: '4px',
                cursor: 'pointer',
              }}
            >
              {branches.map((b: BranchInfo) => (
                <option key={b.id} value={b.isMain ? '' : b.name}>
                  {b.name || '(main)'}{b.isMain ? ' (main)' : ''}
                </option>
              ))}
            </select>
          )}
        </div>

        {project && (
          <div style={{ fontSize: '0.75rem', color: '#888', display: 'flex', gap: '1rem', marginBottom: '12px' }}>
            <span>{project.decisions} decisions</span>
            <span>{project.sections} sections</span>
            <span>{project.pendingTasks} tasks</span>
            {project.openThreads > 0 && <span style={{ color: '#fa4' }}>{project.openThreads} open threads</span>}
            {project.unresolvedConflicts > 0 && <span style={{ color: '#f44' }}>{project.unresolvedConflicts} conflicts</span>}
            {project.staleSections > 0 && <span style={{ color: '#eab308' }}>{project.staleSections} stale</span>}
          </div>
        )}

        {/* Tab bar */}
        <div style={{ display: 'flex', gap: '2px' }}>
          {tabs.map(tab => {
            const isActive = activeTab === tab.key
            return (
              <button key={tab.key} onClick={() => setActiveTab(tab.key)} style={{
                background: isActive ? '#2a2a2a' : 'transparent',
                border: 'none',
                borderBottom: isActive ? '2px solid #4a9eff' : '2px solid transparent',
                color: isActive ? '#e0e0e0' : '#888',
                fontSize: '0.85rem', padding: '8px 14px', cursor: 'pointer',
                display: 'flex', alignItems: 'center', gap: '6px',
                transition: 'all 0.15s', borderRadius: '4px 4px 0 0',
              }}
                onMouseEnter={e => { if (!isActive) e.currentTarget.style.color = '#ccc' }}
                onMouseLeave={e => { if (!isActive) e.currentTarget.style.color = '#888' }}
              >
                {tab.label}
                {tab.count !== undefined && tab.count > 0 && (
                  <span style={{ fontSize: '0.65rem', color: '#555', background: '#252525', padding: '1px 5px', borderRadius: '3px' }}>
                    {tab.count}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </div>

      {/* Content area */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflow: 'hidden' }}>
          {activeTab === 'timeline' && dag && (
            <Timeline
              dag={dag}
              selectedDecisionId={selectedDecisionId}
              onSelectDecision={selectDecision}
              onSelectThread={selectThread}
            />
          )}
          {activeTab === 'state' && dag && <StateView sections={dag.sections || []} />}
          {activeTab === 'threads' && dag && (
            <ThreadList threads={dag.threads || []} onSelectThread={selectThread} selectedThreadId={panelTarget?.kind === 'thread' ? panelTarget.id : null} />
          )}
          {activeTab === 'tasks' && dag && <TaskListInline tasks={dag.tasks || []} />}
          {!dag && <div style={{ padding: '2rem', color: '#888', textAlign: 'center' }}>Loading...</div>}
        </div>

        {panelTarget && id && (
          <DetailPanel
            projectId={id}
            target={panelTarget}
            onClose={() => setPanelTarget(null)}
            onOpenThread={selectThread}
          />
        )}
      </div>
    </div>
  )
}

// Inline task list for Tasks tab
function TaskListInline({ tasks }: { tasks: EntityNode[] }) {
  const grouped: Record<string, EntityNode[]> = {}
  for (const task of tasks) {
    const status = task.status.split(' ')[0]
    if (!grouped[status]) grouped[status] = []
    grouped[status].push(task)
  }
  const statusOrder = ['todo', 'in-progress', 'done', 'cancelled']
  const statusColors: Record<string, string> = { 'todo': '#f97316', 'in-progress': '#fb923c', 'done': '#4c4', 'cancelled': '#888' }

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem' }}>
      <div style={{ fontSize: '0.8rem', color: '#888', marginBottom: '1rem', padding: '0 0 0.75rem', borderBottom: '1px solid #2a2a2a' }}>
        {tasks.length} task{tasks.length !== 1 ? 's' : ''}
      </div>
      {statusOrder.map(status => {
        const items = grouped[status]
        if (!items || items.length === 0) return null
        return (
          <div key={status} style={{ marginBottom: '1.5rem' }}>
            <div style={{ fontSize: '0.75rem', color: statusColors[status] || '#888', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '8px', display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span style={{ width: '8px', height: '8px', borderRadius: '50%', background: statusColors[status] || '#888' }} />
              {status} ({items.length})
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              {items.map(task => (
                <div key={task.id} style={{ padding: '10px 14px', background: '#1e1e1e', borderRadius: '6px', border: '1px solid #333', borderLeft: `3px solid ${statusColors[status] || '#888'}` }}>
                  <div style={{ fontSize: '0.85rem', fontWeight: 500, color: '#e0e0e0' }}>{task.title}</div>
                  <div style={{ fontSize: '0.7rem', color: '#555', marginTop: '4px', fontFamily: 'monospace' }}>{task.id.slice(0, 8)}</div>
                </div>
              ))}
            </div>
          </div>
        )
      })}
      {tasks.length === 0 && (
        <div style={{ textAlign: 'center', color: '#555', padding: '3rem', fontSize: '0.9rem' }}>
          No tasks yet. Create one with <code style={{ color: '#888' }}>orbit task add</code>.
        </div>
      )}
    </div>
  )
}
