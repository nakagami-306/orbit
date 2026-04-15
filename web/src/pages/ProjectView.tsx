import { useEffect, useState, useCallback } from 'react'
import { useParams } from 'react-router-dom'
import {
  ReactFlow,
  Background,
  Controls,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { fetchJSON, type DAGResponse, type ProjectDetail } from '../api/client'
import DecisionNode from '../components/DecisionNode'
import SidePanel from '../components/SidePanel'

const nodeTypes = { decision: DecisionNode }

export default function ProjectView() {
  const { id } = useParams<{ id: string }>()
  const [project, setProject] = useState<ProjectDetail | null>(null)
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
      .then(dag => {
        // Layout: simple top-down by tx order
        const flowNodes: Node[] = dag.nodes.map((n, i) => ({
          id: n.id,
          type: 'decision',
          position: { x: 150, y: i * 120 },
          data: {
            title: n.title,
            author: n.author,
            instant: n.instant,
            type: n.type,
            rationale: n.rationale,
          },
        }))
        const flowEdges: Edge[] = dag.edges.map((e, i) => ({
          id: `e-${i}`,
          source: e.source,
          target: e.target,
          animated: false,
          style: { stroke: '#555' },
        }))
        setNodes(flowNodes)
        setEdges(flowEdges)
      })
      .catch(e => setError(e.message))
  }, [id])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedDecision(node.id)
  }, [])

  if (error) return <div style={{ padding: '2rem', color: '#f66' }}>Error: {error}</div>

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <div style={{ flex: 1, position: 'relative' }}>
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
          }}>
            <h2 style={{ margin: 0, fontSize: '1.1rem' }}>{project.name}</h2>
            <div style={{ fontSize: '0.75rem', color: '#888', marginTop: '4px' }}>
              {project.decisions} decisions &middot; {project.sections} sections &middot; {project.pendingTasks} tasks
            </div>
          </div>
        )}
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onNodeClick={onNodeClick}
          nodeTypes={nodeTypes}
          fitView
          colorMode="dark"
        >
          <Background />
          <Controls />
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
