import { useMemo } from 'react'
import type { GraphResponse, GraphDecision, BranchInfo } from '../api/client'
import { formatTimeShort } from '../utils/time'

interface Props {
  graph: GraphResponse
  onSelectDecision?: (id: string) => void
  selectedDecisionId?: string | null
}

const LANE_W = 28      // horizontal spacing per lane
const ROW_H = 52       // vertical spacing per row
const DOT_R = 5        // dot radius
const LEFT_PAD = 16    // left padding for SVG

// Branch colors — main gets blue, others cycle through these
const LANE_COLORS = [
  '#4a9eff', // main (blue)
  '#a3e635', // lime
  '#f97316', // orange
  '#a855f7', // purple
  '#22c55e', // green
  '#f43f5e', // rose
  '#06b6d4', // cyan
  '#eab308', // yellow
]

interface LayoutRow {
  decision: GraphDecision
  lane: number
  y: number
  branch: BranchInfo
}

export default function BranchGraph({ graph, onSelectDecision, selectedDecisionId }: Props) {
  const { rows, laneCount, laneColors, connections, laneRanges } = useMemo(() => {
    const { decisions, edges, branches } = graph

    // Assign lanes: main = 0, others sorted by first appearance
    const branchIdToLane = new Map<string, number>()
    const branchById = new Map<string, BranchInfo>()
    const branchColors = new Map<string, string>()

    // Sort: main first, then by first decision instant
    const sortedBranches = [...branches].sort((a, b) => {
      if (a.isMain) return -1
      if (b.isMain) return 1
      return 0
    })

    sortedBranches.forEach((b, i) => {
      branchIdToLane.set(b.id, i)
      branchById.set(b.id, b)
      branchColors.set(b.id, LANE_COLORS[i % LANE_COLORS.length])
    })

    const totalLanes = sortedBranches.length

    // Build lookup: decision id -> index in sorted list
    const idToDecision = new Map<string, GraphDecision>()
    for (const d of decisions) {
      idToDecision.set(d.id, d)
    }

    // Build parent map
    const parentMap = new Map<string, string[]>()
    for (const e of edges) {
      const parents = parentMap.get(e.target) || []
      parents.push(e.source)
      parentMap.set(e.target, parents)
    }

    // Sort decisions by instant (newest first)
    const sorted = [...decisions].sort((a, b) =>
      (b.instant || '').localeCompare(a.instant || '')
    )

    // Build layout rows
    const layoutRows: LayoutRow[] = sorted.map((d, i) => ({
      decision: d,
      lane: branchIdToLane.get(d.branchId) ?? 0,
      y: i * ROW_H + ROW_H / 2,
      branch: branchById.get(d.branchId) || sortedBranches[0],
    }))

    const idToRow = new Map<string, LayoutRow>()
    for (const row of layoutRows) {
      idToRow.set(row.decision.id, row)
    }

    // Build connections (edges with coordinates)
    type Connection = {
      fromX: number; fromY: number
      toX: number; toY: number
      color: string
      isCrossLane: boolean
    }
    const conns: Connection[] = []

    for (const e of edges) {
      const fromRow = idToRow.get(e.source)
      const toRow = idToRow.get(e.target)
      if (!fromRow || !toRow) continue

      const fromX = LEFT_PAD + fromRow.lane * LANE_W
      const fromY = fromRow.y
      const toX = LEFT_PAD + toRow.lane * LANE_W
      const toY = toRow.y
      const color = branchColors.get(toRow.decision.branchId) || LANE_COLORS[0]

      conns.push({
        fromX, fromY, toX, toY,
        color,
        isCrossLane: fromRow.lane !== toRow.lane,
      })
    }

    // Compute lane active ranges (first row index to last row index per lane)
    const ranges = new Map<number, { startY: number; endY: number; color: string }>()
    for (const row of layoutRows) {
      const existing = ranges.get(row.lane)
      if (!existing) {
        ranges.set(row.lane, {
          startY: row.y,
          endY: row.y,
          color: branchColors.get(row.decision.branchId) || LANE_COLORS[0],
        })
      } else {
        existing.endY = row.y
      }
    }

    return {
      rows: layoutRows,
      laneCount: totalLanes,
      laneColors: branchColors,
      connections: conns,
      laneRanges: ranges,
    }
  }, [graph])

  const svgWidth = LEFT_PAD + laneCount * LANE_W + 8
  const svgHeight = rows.length * ROW_H

  return (
    <div style={{ height: '100%', overflow: 'auto', padding: '1rem 0' }}>
      {/* Branch legend */}
      <div style={{
        display: 'flex', gap: '12px', padding: '0 1rem 0.75rem',
        borderBottom: '1px solid #2a2a2a', marginBottom: '0.5rem',
        flexWrap: 'wrap', alignItems: 'center',
      }}>
        <span style={{ fontSize: '0.8rem', color: '#888' }}>
          {rows.length} decision{rows.length !== 1 ? 's' : ''} across {laneCount} branch{laneCount !== 1 ? 'es' : ''}
        </span>
        {graph.branches.map((b, i) => (
          <span key={b.id} style={{
            fontSize: '0.7rem', display: 'flex', alignItems: 'center', gap: '4px',
          }}>
            <span style={{
              width: 10, height: 10, borderRadius: '50%',
              background: LANE_COLORS[i % LANE_COLORS.length],
              display: 'inline-block',
            }} />
            <span style={{ color: '#ccc' }}>{b.name || 'main'}</span>
            {b.isMain && <span style={{ color: '#555', fontSize: '0.6rem' }}>(main)</span>}
          </span>
        ))}
      </div>

      {/* Graph + cards */}
      <div style={{ position: 'relative', display: 'flex' }}>
        {/* SVG graph column */}
        <svg
          width={svgWidth}
          height={svgHeight}
          style={{ flexShrink: 0, display: 'block' }}
        >
          {/* Vertical lane lines (background rails) */}
          {Array.from(laneRanges.entries()).map(([lane, range]) => (
            <line
              key={`rail-${lane}`}
              x1={LEFT_PAD + lane * LANE_W}
              y1={range.startY}
              x2={LEFT_PAD + lane * LANE_W}
              y2={range.endY}
              stroke={range.color}
              strokeWidth={2}
              strokeOpacity={0.3}
            />
          ))}

          {/* Edge connections */}
          {connections.map((c, i) => {
            if (!c.isCrossLane) {
              // Same lane: straight line
              return (
                <line
                  key={`edge-${i}`}
                  x1={c.fromX} y1={c.fromY}
                  x2={c.toX} y2={c.toY}
                  stroke={c.color}
                  strokeWidth={2}
                  strokeOpacity={0.6}
                />
              )
            }
            // Cross-lane: curved path
            const midY = c.fromY + (c.toY - c.fromY) * 0.3
            return (
              <path
                key={`edge-${i}`}
                d={`M ${c.fromX} ${c.fromY} C ${c.fromX} ${midY}, ${c.toX} ${midY}, ${c.toX} ${c.toY}`}
                fill="none"
                stroke={c.color}
                strokeWidth={2}
                strokeOpacity={0.6}
              />
            )
          })}

          {/* Decision dots */}
          {rows.map(row => {
            const x = LEFT_PAD + row.lane * LANE_W
            const color = laneColors.get(row.decision.branchId) || LANE_COLORS[0]
            const isSelected = selectedDecisionId === row.decision.id
            const isHead = graph.branches.some(b =>
              b.headDecisionId === row.decision.id
            )
            const r = isHead ? DOT_R + 2 : DOT_R
            return (
              <g key={row.decision.id}>
                {isSelected && (
                  <circle cx={x} cy={row.y} r={r + 4} fill="none" stroke={color} strokeWidth={1.5} strokeOpacity={0.4} />
                )}
                <circle
                  cx={x} cy={row.y} r={r}
                  fill={color}
                  stroke={isSelected ? '#fff' : 'none'}
                  strokeWidth={isSelected ? 2 : 0}
                  style={{ cursor: 'pointer' }}
                  onClick={() => onSelectDecision?.(row.decision.id)}
                />
                {isHead && (
                  <text x={x} y={row.y - r - 4} textAnchor="middle" fontSize="8" fill={color} fontWeight="bold">
                    HEAD
                  </text>
                )}
              </g>
            )
          })}
        </svg>

        {/* Decision cards */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {rows.map(row => {
            const color = laneColors.get(row.decision.branchId) || LANE_COLORS[0]
            const isSelected = selectedDecisionId === row.decision.id
            return (
              <div
                key={row.decision.id}
                onClick={() => onSelectDecision?.(row.decision.id)}
                style={{
                  height: ROW_H,
                  display: 'flex',
                  alignItems: 'center',
                  padding: '0 12px',
                  cursor: 'pointer',
                  background: isSelected ? '#141c28' : 'transparent',
                  borderRadius: '4px',
                  transition: 'background 0.1s',
                }}
                onMouseEnter={e => { if (!isSelected) e.currentTarget.style.background = '#181818' }}
                onMouseLeave={e => { if (!isSelected) e.currentTarget.style.background = 'transparent' }}
              >
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <span style={{
                      fontSize: '0.85rem', fontWeight: 600, color: '#e0e0e0',
                      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}>
                      {row.decision.title}
                    </span>
                    {row.decision.type === 'merge' && (
                      <span style={{
                        fontSize: '0.55rem', padding: '1px 4px', borderRadius: '3px',
                        background: '#2a1a3a', color: '#a855f7', flexShrink: 0,
                      }}>merge</span>
                    )}
                    {row.decision.type === 'root' && (
                      <span style={{
                        fontSize: '0.55rem', padding: '1px 4px', borderRadius: '3px',
                        background: '#1a2a3a', color: '#4a9eff', flexShrink: 0,
                      }}>root</span>
                    )}
                  </div>
                  <div style={{
                    fontSize: '0.7rem', color: '#666', marginTop: '2px',
                    display: 'flex', gap: '8px', alignItems: 'center',
                  }}>
                    <span style={{
                      color, fontSize: '0.6rem', padding: '0 4px',
                      background: `${color}15`, borderRadius: '2px',
                    }}>
                      {row.branch.name || 'main'}
                    </span>
                    <span>{row.decision.author}</span>
                    <span>{formatTimeShort(row.decision.instant)}</span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {rows.length === 0 && (
        <div style={{ textAlign: 'center', color: '#555', padding: '3rem', fontSize: '0.9rem' }}>
          No decisions yet.
        </div>
      )}
    </div>
  )
}
