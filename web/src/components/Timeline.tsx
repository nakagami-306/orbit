import { useMemo, useState } from 'react'
import type { DAGResponse, DAGNode, EntityNode } from '../api/client'
import TimelineNode from './TimelineNode'

interface Props {
  dag: DAGResponse
  selectedDecisionId: string | null
  onSelectDecision: (id: string | null) => void
}

interface ProcessedDecision {
  decision: DAGNode
  relatedThreads: EntityNode[]
  relatedTasks: EntityNode[]
  relatedSections: EntityNode[]
  milestone: string | null
  sourceThread: EntityNode | null
  parentCount: number
  isRoot: boolean
}

export default function Timeline({ dag, selectedDecisionId, onSelectDecision }: Props) {
  const [sortNewestFirst, setSortNewestFirst] = useState(true)

  const processed = useMemo(() => {
    // Build entity lookup
    const entityMap = new Map<string, EntityNode>()
    for (const e of [...(dag.threads || []), ...(dag.tasks || []), ...(dag.sections || [])]) {
      entityMap.set(e.id, e)
    }

    // Build milestone lookup (decisionId -> title)
    const milestoneMap = new Map<string, string>()
    for (const ms of dag.milestones || []) {
      milestoneMap.set(ms.decisionId, ms.title)
    }

    // Build entity edges: decision -> related entities
    // entityEdges go from decision to entity (source=decision, target=entity)
    const decisionEntities = new Map<string, EntityNode[]>()
    for (const edge of dag.entityEdges || []) {
      const entity = entityMap.get(edge.target)
      if (entity) {
        const existing = decisionEntities.get(edge.source) || []
        existing.push(entity)
        decisionEntities.set(edge.source, existing)
      }
      // Also check reverse direction
      const entityReverse = entityMap.get(edge.source)
      if (entityReverse) {
        const existing = decisionEntities.get(edge.target) || []
        existing.push(entityReverse)
        decisionEntities.set(edge.target, existing)
      }
    }

    // Build parent count (how many edges target this decision)
    const parentCount = new Map<string, number>()
    const childCount = new Map<string, number>()
    for (const edge of dag.edges || []) {
      parentCount.set(edge.target, (parentCount.get(edge.target) || 0) + 1)
      childCount.set(edge.source, (childCount.get(edge.source) || 0) + 1)
    }

    // Find root decisions (no parents among decision edges)
    const decisionIds = new Set(dag.nodes.map(n => n.id))
    const hasParent = new Set<string>()
    for (const edge of dag.edges || []) {
      if (decisionIds.has(edge.target)) {
        hasParent.add(edge.target)
      }
    }

    // Process each decision
    const result: ProcessedDecision[] = dag.nodes.map(decision => {
      const related = decisionEntities.get(decision.id) || []
      const relatedThreads = related.filter(e => e.type === 'thread')
      const relatedTasks = related.filter(e => e.type === 'task')
      const relatedSections = related.filter(e => e.type === 'section')

      // Find source thread
      let sourceThread: EntityNode | null = null
      if (decision.sourceThreadId) {
        sourceThread = entityMap.get(decision.sourceThreadId) || null
      }

      return {
        decision,
        relatedThreads,
        relatedTasks,
        relatedSections,
        milestone: milestoneMap.get(decision.id) || null,
        sourceThread,
        parentCount: parentCount.get(decision.id) || 0,
        isRoot: !hasParent.has(decision.id),
      }
    })

    // Sort by instant (chronological)
    result.sort((a, b) => {
      const aTime = a.decision.instant || ''
      const bTime = b.decision.instant || ''
      return sortNewestFirst
        ? bTime.localeCompare(aTime)
        : aTime.localeCompare(bTime)
    })

    return result
  }, [dag, sortNewestFirst])

  // Branch info
  const branches = dag.branches || []
  const mainBranch = branches.find(b => b.isMain)
  const otherBranches = branches.filter(b => !b.isMain)

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem' }}>
      {/* Controls */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: '1rem',
        padding: '0 0 0.75rem',
        borderBottom: '1px solid #2a2a2a',
      }}>
        <div style={{ fontSize: '0.8rem', color: '#888' }}>
          {processed.length} decision{processed.length !== 1 ? 's' : ''}
          {branches.length > 1 && (
            <span style={{ marginLeft: '12px' }}>
              {branches.length} branch{branches.length !== 1 ? 'es' : ''}
            </span>
          )}
        </div>

        <button
          onClick={() => setSortNewestFirst(prev => !prev)}
          style={{
            background: '#2a2a2a',
            border: '1px solid #444',
            color: '#aaa',
            fontSize: '0.75rem',
            padding: '4px 10px',
            borderRadius: '4px',
            cursor: 'pointer',
          }}
          onMouseEnter={e => { e.currentTarget.style.background = '#333' }}
          onMouseLeave={e => { e.currentTarget.style.background = '#2a2a2a' }}
        >
          {sortNewestFirst ? 'Newest first' : 'Oldest first'}
        </button>
      </div>

      {/* Branch indicators */}
      {branches.length > 1 && (
        <div style={{
          display: 'flex',
          gap: '8px',
          marginBottom: '1rem',
          flexWrap: 'wrap',
        }}>
          {mainBranch && (
            <span style={{
              fontSize: '0.7rem',
              padding: '2px 8px',
              borderRadius: '4px',
              background: '#1a2a3a',
              color: '#4a9eff',
              border: '1px solid #2a3a4a',
            }}>
              {mainBranch.name || 'main'}
            </span>
          )}
          {otherBranches.map(b => (
            <span key={b.id} style={{
              fontSize: '0.7rem',
              padding: '2px 8px',
              borderRadius: '4px',
              background: '#2a2a2a',
              color: '#888',
              border: '1px solid #333',
            }}>
              {b.name || '(unnamed)'}
              {b.status !== 'active' && (
                <span style={{ marginLeft: '4px', color: '#555' }}>({b.status})</span>
              )}
            </span>
          ))}
        </div>
      )}

      {/* Timeline */}
      <div style={{ position: 'relative' }}>
        {processed.map(item => (
          <TimelineNode
            key={item.decision.id}
            decision={item.decision}
            isSelected={selectedDecisionId === item.decision.id}
            onClick={() => {
              onSelectDecision(
                selectedDecisionId === item.decision.id ? null : item.decision.id
              )
            }}
            relatedThreads={item.relatedThreads}
            relatedTasks={item.relatedTasks}
            relatedSections={item.relatedSections}
            milestone={item.milestone}
            sourceThread={item.sourceThread}
            parentCount={item.parentCount}
            isRoot={item.isRoot}
          />
        ))}

        {processed.length === 0 && (
          <div style={{
            textAlign: 'center',
            color: '#555',
            padding: '3rem',
            fontSize: '0.9rem',
          }}>
            No decisions yet. Create one with <code style={{ color: '#888' }}>orbit decide</code>.
          </div>
        )}
      </div>
    </div>
  )
}
