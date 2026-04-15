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
import { fetchJSON, type DAGResponse, type ProjectDetail, type MilestoneInfo, type EntityNode } from '../api/client'
import DecisionNode from '../components/DecisionNode'
import ThreadNode from '../components/ThreadNode'
import TaskNode from '../components/TaskNode'
import SectionNode from '../components/SectionNode'
import SidePanel from '../components/SidePanel'

const nodeTypes = {
  decision: DecisionNode,
  thread: ThreadNode,
  task: TaskNode,
  section: SectionNode,
}

const DECISION_NODE_WIDTH = 260
const DECISION_NODE_HEIGHT = 60
const ENTITY_NODE_WIDTH = 180
const ENTITY_NODE_HEIGHT = 40

interface SelectedNode {
  id: string
  type: 'decision' | 'thread' | 'task' | 'section'
  data?: EntityNode
}

function layoutDAG(
  dagNodes: DAGResponse['nodes'],
  dagEdges: DAGResponse['edges'],
  milestones: MilestoneInfo[],
  threads: EntityNode[],
  tasks: EntityNode[],
  sections: EntityNode[],
  entityEdges: DAGResponse['entityEdges'],
) {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'TB', nodesep: 60, ranksep: 80 })

  const milestoneMap = new Map<string, string>()
  for (const ms of milestones) {
    milestoneMap.set(ms.decisionId, ms.title)
  }

  // Decision nodes
  dagNodes.forEach(n => {
    g.setNode(n.id, { width: DECISION_NODE_WIDTH, height: DECISION_NODE_HEIGHT })
  })

  // Entity nodes (smaller)
  const allEntities = [...threads, ...tasks, ...sections]
  allEntities.forEach(n => {
    g.setNode(n.id, { width: ENTITY_NODE_WIDTH, height: ENTITY_NODE_HEIGHT })
  })

  // Decision parent edges
  dagEdges.forEach(e => {
    g.setEdge(e.source, e.target)
  })

  // Entity edges
  entityEdges.forEach(e => {
    // Only add edge if both nodes exist in the graph
    if (g.hasNode(e.source) && g.hasNode(e.target)) {
      g.setEdge(e.source, e.target)
    }
  })

  dagre.layout(g)

  // Build flow nodes: decisions
  const flowNodes: Node[] = dagNodes.map(n => {
    const pos = g.node(n.id)
    return {
      id: n.id,
      type: 'decision',
      position: { x: pos.x - DECISION_NODE_WIDTH / 2, y: pos.y - DECISION_NODE_HEIGHT / 2 },
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

  // Entity nodes
  allEntities.forEach(n => {
    const pos = g.node(n.id)
    if (!pos) return
    flowNodes.push({
      id: n.id,
      type: n.type,
      position: { x: pos.x - ENTITY_NODE_WIDTH / 2, y: pos.y - ENTITY_NODE_HEIGHT / 2 },
      data: {
        title: n.title,
        status: n.status,
        instant: n.instant,
      },
    })
  })

  // Decision parent edges
  let edgeIdx = 0
  const flowEdges: Edge[] = dagEdges.map(e => ({
    id: `e-${edgeIdx++}`,
    source: e.source,
    target: e.target,
    type: 'smoothstep',
    animated: false,
    style: { stroke: '#555', strokeWidth: 1.5 },
    markerEnd: { type: MarkerType.ArrowClosed, color: '#555', width: 12, height: 12 },
  }))

  // Entity edges (dashed)
  entityEdges.forEach(e => {
    if (g.hasNode(e.source) && g.hasNode(e.target)) {
      flowEdges.push({
        id: `ee-${edgeIdx++}`,
        source: e.source,
        target: e.target,
        type: 'smoothstep',
        animated: false,
        style: { stroke: '#666', strokeWidth: 1, strokeDasharray: '5,5' },
        markerEnd: { type: MarkerType.ArrowClosed, color: '#666', width: 10, height: 10 },
      })
    }
  })

  return { flowNodes, flowEdges }
}

export default function ProjectView() {
  const { id } = useParams<{ id: string }>()
  const [project, setProject] = useState<ProjectDetail | null>(null)
  const [dag, setDag] = useState<DAGResponse | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [selectedNode, setSelectedNode] = useState<SelectedNode | null>(null)
  const [error, setError] = useState('')

  // Build a lookup map for entity nodes from DAG data
  const entityMap = new Map<string, EntityNode>()
  if (dag) {
    for (const n of [...dag.threads, ...dag.tasks, ...dag.sections]) {
      entityMap.set(n.id, n)
    }
  }

  useEffect(() => {
    if (!id) return
    fetchJSON<ProjectDetail>(`/api/projects/${id}`)
      .then(setProject)
      .catch(e => setError(e.message))

    fetchJSON<DAGResponse>(`/api/projects/${id}/dag`)
      .then(data => {
        setDag(data)
        const { flowNodes, flowEdges } = layoutDAG(
          data.nodes,
          data.edges,
          data.milestones,
          data.threads || [],
          data.tasks || [],
          data.sections || [],
          data.entityEdges || [],
        )
        setNodes(flowNodes)
        setEdges(flowEdges)
      })
      .catch(e => setError(e.message))
  }, [id])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    const nodeType = node.type as 'decision' | 'thread' | 'task' | 'section'
    setSelectedNode(prev => {
      if (prev && prev.id === node.id) return null
      if (nodeType === 'decision') {
        return { id: node.id, type: 'decision' }
      }
      // For entity nodes, pass the entity data
      const entityData = entityMap.get(node.id)
      return { id: node.id, type: nodeType, data: entityData }
    })
  }, [entityMap])

  const onPaneClick = useCallback(() => {
    setSelectedNode(null)
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
            right: selectedNode ? '416px' : '1rem',
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
              const nodeType = n.type
              if (nodeType === 'thread') return '#22c55e'
              if (nodeType === 'task') return '#f97316'
              if (nodeType === 'section') return '#999'
              const d = n.data as Record<string, unknown>
              if (d?.type === 'root') return '#4a9eff'
              if (d?.type === 'merge') return '#a855f7'
              return '#666'
            }}
            style={{ background: '#1a1a1a', border: '1px solid #333' }}
          />
        </ReactFlow>
      </div>

      {selectedNode?.type === 'decision' ? (
        <SidePanel
          projectId={id || ''}
          decisionId={selectedNode.id}
          onClose={() => setSelectedNode(null)}
        />
      ) : selectedNode ? (
        <EntitySidePanel
          node={selectedNode}
          onClose={() => setSelectedNode(null)}
        />
      ) : null}
    </div>
  )
}

// Side panel for entity nodes (Thread, Task, Section)
function EntitySidePanel({ node, onClose }: { node: SelectedNode; onClose: () => void }) {
  const typeLabels: Record<string, string> = {
    thread: 'Thread',
    task: 'Task',
    section: 'Section',
  }
  const typeColors: Record<string, string> = {
    thread: '#22c55e',
    task: '#f97316',
    section: '#999',
  }

  const d = node.data

  return (
    <div style={{
      width: '400px',
      borderLeft: '1px solid #333',
      background: '#1e1e1e',
      padding: '1rem',
      overflow: 'auto',
      flexShrink: 0,
    }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ margin: 0, fontSize: '1rem', color: typeColors[node.type] || '#ccc' }}>
          {typeLabels[node.type] || node.type}
        </h3>
        <button onClick={onClose} style={{
          background: '#333', border: 'none', color: '#888', cursor: 'pointer',
          fontSize: '0.8rem', padding: '4px 8px', borderRadius: '4px',
        }}>ESC</button>
      </div>

      {d ? (
        <>
          <h4 style={{ margin: '0 0 0.5rem', fontSize: '1rem', lineHeight: 1.3 }}>{d.title}</h4>

          <div style={{ fontSize: '0.7rem', color: '#555', marginBottom: '0.75rem', fontFamily: 'monospace' }}>
            {d.id.slice(0, 8)}
          </div>

          <div style={{ marginBottom: '1rem' }}>
            <div style={{ fontSize: '0.7rem', color: '#666', marginBottom: '6px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Status
            </div>
            <StatusBadge status={d.status} type={node.type} />
          </div>

          {d.instant && (
            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontSize: '0.7rem', color: '#666', marginBottom: '6px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Created
              </div>
              <div style={{ fontSize: '0.85rem', color: '#ccc' }}>
                {d.instant.slice(0, 19).replace('T', ' ')}
              </div>
            </div>
          )}
        </>
      ) : (
        <div style={{ color: '#888', fontSize: '0.85rem' }}>
          No details available for this node.
        </div>
      )}
    </div>
  )
}

function StatusBadge({ status, type }: { status: string; type: string }) {
  const colorMap: Record<string, Record<string, { bg: string; fg: string }>> = {
    thread: {
      open: { bg: '#0f2a1a', fg: '#22c55e' },
      decided: { bg: '#1a2a1a', fg: '#16a34a' },
      closed: { bg: '#333', fg: '#888' },
    },
    task: {
      todo: { bg: '#2a1a0a', fg: '#f97316' },
      'in-progress': { bg: '#2a200a', fg: '#fb923c' },
      done: { bg: '#1a2a1a', fg: '#4c4' },
      cancelled: { bg: '#2a1a1a', fg: '#f44' },
    },
    section: {
      current: { bg: '#252525', fg: '#999' },
      stale: { bg: '#2a2510', fg: '#eab308' },
    },
  }
  const baseStatus = status.split(' ')[0]
  const c = colorMap[type]?.[baseStatus] || { bg: '#333', fg: '#aaa' }
  return (
    <span style={{
      fontSize: '0.75rem',
      padding: '2px 8px',
      borderRadius: '4px',
      background: c.bg,
      color: c.fg,
    }}>
      {status}
    </span>
  )
}
