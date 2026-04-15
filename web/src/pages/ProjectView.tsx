import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  MarkerType,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import dagre from '@dagrejs/dagre'
import { fetchJSON, type DAGResponse, type ProjectDetail, type MilestoneInfo } from '../api/client'
import DecisionNode from '../components/DecisionNode'
import SidePanel from '../components/SidePanel'

const nodeTypes = { decision: DecisionNode }

const NODE_WIDTH = 260
const NODE_HEIGHT = 60

function layoutDAG(dagNodes: DAGResponse['nodes'], dagEdges: DAGResponse['edges'], milestones: MilestoneInfo[]) {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'TB', nodesep: 60, ranksep: 80 })

  const milestoneMap = new Map<string, string>()
  for (const ms of milestones) {
    milestoneMap.set(ms.decisionId, ms.title)
  }

  dagNodes.forEach(n => {
    g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT })
  })
  dagEdges.forEach(e => {
    g.setEdge(e.source, e.target)
  })

  dagre.layout(g)

  const flowNodes: Node[] = dagNodes.map(n => {
    const pos = g.node(n.id)
    return {
      id: n.id,
      type: 'decision',
      position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 },
      data: {
        title: n.title,
        author: n.author,
        instant: n.instant,
        type: n.type,
        rationale: n.rationale,
        milestone: milestoneMap.get(n.id) || null,
        sourceThreadId: n.sourceThreadId,
      },
    }
  })

  const flowEdges: Edge[] = dagEdges.map((e, i) => ({
    id: `e-${i}`,
    source: e.source,
    target: e.target,
    type: 'smoothstep',
    animated: false,
    style: { stroke: '#555', strokeWidth: 1.5 },
    markerEnd: { type: MarkerType.ArrowClosed, color: '#555', width: 12, height: 12 },
  }))

  return { flowNodes, flowEdges }
}

export default function ProjectView() {
  const { id } = useParams<{ id: string }>()
  const [project, setProject] = useState<ProjectDetail | null>(null)
  const [dag, setDag] = useState<DAGResponse | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [selectedDecision, setSelectedDecision] = useState<string | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    fetchJSON<ProjectDetail>(`/api/projects/${id}`)
      .then(setProject)
      .catch(e => setError(e.message))

    fetchJSON<DAGResponse>(`/api/projects/${id}/dag`)
      .then(data => {
        setDag(data)
        const { flowNodes, flowEdges } = layoutDAG(data.nodes, data.edges, data.milestones)
        setNodes(flowNodes)
        setEdges(flowEdges)
      })
      .catch(e => setError(e.message))
  }, [id])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedDecision(prev => prev === node.id ? null : node.id)
  }, [])

  const onPaneClick = useCallback(() => {
    setSelectedDecision(null)
  }, [])

  if (error) return <div style={{ padding: '2rem', color: '#f66' }}>Error: {error}</div>

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <div style={{ flex: 1, position: 'relative' }}>
        {/* Project info overlay */}
        {project && (
          <div style={{
            position: 'absolute',
            top: '1rem',
            left: '1rem',
            zIndex: 10,
            background: '#1a1a1aee',
            padding: '0.75rem 1rem',
            borderRadius: '8px',
            border: '1px solid #333',
            backdropFilter: 'blur(8px)',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Link to="/" style={{ color: '#888', textDecoration: 'none', fontSize: '0.8rem' }}>Projects</Link>
              <span style={{ color: '#555' }}>/</span>
              <h2 style={{ margin: 0, fontSize: '1.1rem' }}>{project.name}</h2>
            </div>
            <div style={{ fontSize: '0.75rem', color: '#888', marginTop: '4px', display: 'flex', gap: '0.75rem' }}>
              <span>{project.decisions} decisions</span>
              <span>{project.sections} sections</span>
              <span>{project.pendingTasks} tasks</span>
              {project.openThreads > 0 && <span style={{ color: '#fa4' }}>{project.openThreads} open threads</span>}
              {project.unresolvedConflicts > 0 && <span style={{ color: '#f44' }}>{project.unresolvedConflicts} conflicts</span>}
            </div>
          </div>
        )}

        {/* Branch info */}
        {dag && dag.branches.length > 1 && (
          <div style={{
            position: 'absolute',
            top: '1rem',
            right: selectedDecision ? '416px' : '1rem',
            zIndex: 10,
            background: '#1a1a1aee',
            padding: '0.5rem 0.75rem',
            borderRadius: '6px',
            border: '1px solid #333',
            fontSize: '0.75rem',
            transition: 'right 0.2s',
          }}>
            {dag.branches.map(b => (
              <div key={b.id} style={{ color: b.isMain ? '#4a9eff' : '#888', marginBottom: '2px' }}>
                {b.name || '(unnamed)'} {b.isMain && '●'}
              </div>
            ))}
          </div>
        )}

        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onNodeClick={onNodeClick}
          onPaneClick={onPaneClick}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          colorMode="dark"
          minZoom={0.2}
          maxZoom={2}
        >
          <Background gap={20} />
          <Controls />
          <MiniMap
            nodeColor={(n) => {
              const d = n.data as any
              if (d?.type === 'root') return '#4a9eff'
              if (d?.type === 'merge') return '#a855f7'
              return '#666'
            }}
            style={{ background: '#1a1a1a', border: '1px solid #333' }}
          />
        </ReactFlow>
      </div>

      <SidePanel
        projectId={id || ''}
        decisionId={selectedDecision}
        onClose={() => setSelectedDecision(null)}
      />
    </div>
  )
}
