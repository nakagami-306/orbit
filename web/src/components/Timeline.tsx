import { useMemo, useState } from 'react'
import type { DAGResponse, DAGNode, EntityNode } from '../api/client'
import TimelineNode from './TimelineNode'

interface Props {
  dag: DAGResponse
  selectedDecisionId: string | null
  onSelectDecision: (id: string | null) => void
  onSelectThread?: (threadId: string) => void
  headDecisionId?: string | null
  onSwitchBranch?: (branchName: string) => void
  currentBranch?: string
}

export interface ProcessedDecision {
  decision: DAGNode
  relatedThreads: EntityNode[]
  relatedTasks: EntityNode[]
  relatedSections: EntityNode[]
  milestone: string | null
  sourceThread: EntityNode | null
  parentCount: number
  isRoot: boolean
  parentTitles: string[]
  childCount: number
}

export default function Timeline({ dag, selectedDecisionId, onSelectDecision, onSelectThread, headDecisionId, onSwitchBranch, currentBranch }: Props) {
  const [sortNewestFirst, setSortNewestFirst] = useState(true)

  const processed = useMemo(() => {
    // Build entity lookup
    const entityMap = new Map<string, EntityNode>()
    for (const e of [...(dag.threads || []), ...(dag.tasks || []), ...(dag.sections || [])]) {
      entityMap.set(e.id, e)
    }

    // Build decision title lookup
    const decisionTitleMap = new Map<string, string>()
    for (const n of dag.nodes) {
      decisionTitleMap.set(n.id, n.title)
    }

    // Build milestone lookup (decisionId -> title)
    const milestoneMap = new Map<string, string>()
    for (const ms of dag.milestones || []) {
      milestoneMap.set(ms.decisionId, ms.title)
    }

    // Build entity edges: decision -> related entities
    const decisionEntities = new Map<string, EntityNode[]>()
    for (const edge of dag.entityEdges || []) {
      const entity = entityMap.get(edge.target)
      if (entity) {
        const existing = decisionEntities.get(edge.source) || []
        existing.push(entity)
        decisionEntities.set(edge.source, existing)
      }
      // Also check reverse direction (thread -> decision)
      const entityReverse = entityMap.get(edge.source)
      if (entityReverse) {
        const existing = decisionEntities.get(edge.target) || []
        existing.push(entityReverse)
        decisionEntities.set(edge.target, existing)
      }
    }

    // Build parent/child relationships
    const parentIds = new Map<string, string[]>()
    const childCounts = new Map<string, number>()
    for (const edge of dag.edges || []) {
      const parents = parentIds.get(edge.target) || []
      parents.push(edge.source)
      parentIds.set(edge.target, parents)
      childCounts.set(edge.source, (childCounts.get(edge.source) || 0) + 1)
    }

    // Find root decisions
    const hasParent = new Set<string>()
    for (const edge of dag.edges || []) {
      hasParent.add(edge.target)
    }

    // Process each decision
    const result: ProcessedDecision[] = dag.nodes.map(decision => {
      const related = decisionEntities.get(decision.id) || []
      const relatedThreads = related.filter(e => e.type === 'thread')
      const relatedTasks = related.filter(e => e.type === 'task')
      const relatedSections = related.filter(e => e.type === 'section')

      let sourceThread: EntityNode | null = null
      if (decision.sourceThreadId) {
        sourceThread = entityMap.get(decision.sourceThreadId) || null
      }

      const parents = parentIds.get(decision.id) || []
      const parentTitles = parents.map(pid => decisionTitleMap.get(pid) || pid)

      return {
        decision,
        relatedThreads,
        relatedTasks,
        relatedSections,
        milestone: milestoneMap.get(decision.id) || null,
        sourceThread,
        parentCount: parents.length,
        isRoot: !hasParent.has(decision.id),
        parentTitles,
        childCount: childCounts.get(decision.id) || 0,
      }
    })

    result.sort((a, b) => {
      const aTime = a.decision.instant || ''
      const bTime = b.decision.instant || ''
      return sortNewestFirst
        ? bTime.localeCompare(aTime)
        : aTime.localeCompare(bTime)
    })

    return result
  }, [dag, sortNewestFirst])

  const branches = dag.branches || []

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem 1.5rem' }}>
      {/* Controls */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: '1rem',
        padding: '0 0 0.75rem',
        borderBottom: '1px solid #2a2a2a',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div style={{ fontSize: '0.8rem', color: '#888' }}>
            {processed.length} decision{processed.length !== 1 ? 's' : ''}
          </div>
          {/* Branch indicators inline — clickable to switch */}
          {branches.length > 1 && (
            <div style={{ display: 'flex', gap: '6px' }}>
              {branches.map(b => {
                const name = b.name || 'main'
                const branchValue = b.isMain ? '' : b.name
                const isActive = b.isMain ? !currentBranch : currentBranch === b.name
                return (
                  <span
                    key={b.id}
                    onClick={() => onSwitchBranch?.(branchValue)}
                    style={{
                      fontSize: '0.65rem', padding: '1px 6px', borderRadius: '3px',
                      background: isActive ? '#1a2a3a' : '#2a2a2a',
                      color: isActive ? '#4a9eff' : '#888',
                      border: isActive ? '1px solid #2a3a4a' : '1px solid #333',
                      cursor: onSwitchBranch ? 'pointer' : 'default',
                    }}
                  >
                    {name}
                  </span>
                )
              })}
            </div>
          )}
        </div>

        <button
          onClick={() => setSortNewestFirst(prev => !prev)}
          style={{
            background: '#2a2a2a', border: '1px solid #444', color: '#aaa',
            fontSize: '0.75rem', padding: '4px 10px', borderRadius: '4px', cursor: 'pointer',
          }}
          onMouseEnter={e => { e.currentTarget.style.background = '#333' }}
          onMouseLeave={e => { e.currentTarget.style.background = '#2a2a2a' }}
        >
          {sortNewestFirst ? 'Newest first' : 'Oldest first'}
        </button>
      </div>

      {/* Timeline */}
      <div style={{ position: 'relative', paddingLeft: '20px' }}>
        {/* Continuous vertical line */}
        {processed.length > 0 && (
          <div style={{
            position: 'absolute',
            left: '20px',
            top: 0,
            bottom: 0,
            width: '2px',
            background: '#2a2a2a',
          }} />
        )}

        {processed.map((item, idx) => (
          <TimelineNode
            key={item.decision.id}
            item={item}
            isSelected={selectedDecisionId === item.decision.id}
            onClick={() => {
              onSelectDecision(
                selectedDecisionId === item.decision.id ? null : item.decision.id
              )
            }}
            onClickThread={onSelectThread}
            isFirst={idx === 0}
            isLast={idx === processed.length - 1}
            isHead={headDecisionId === item.decision.id}
          />
        ))}

        {processed.length === 0 && (
          <div style={{
            textAlign: 'center', color: '#555', padding: '3rem', fontSize: '0.9rem',
          }}>
            No decisions yet. Create one with <code style={{ color: '#888' }}>orbit decide</code>.
          </div>
        )}
      </div>
    </div>
  )
}
