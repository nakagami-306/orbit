const BASE = ''

export async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export async function patchJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return res.json()
}

// --- Types ---

export interface Project {
  id: string
  name: string
  description: string
  status: string
}

export interface ProjectDetail extends Project {
  sections: number
  staleSections: number
  decisions: number
  openThreads: number
  unresolvedConflicts: number
  pendingTasks: number
}

export interface DAGNode {
  id: string
  title: string
  rationale: string
  author: string
  instant: string
  sourceThreadId: string | null
  type: 'root' | 'normal' | 'merge'
}

export interface DAGEdge {
  source: string
  target: string
}

export interface BranchInfo {
  id: string
  name: string
  isMain: boolean
  headDecisionId: string | null
  status: string
}

export interface MilestoneInfo {
  id: string
  title: string
  decisionId: string
}

export interface EntityNode {
  id: string
  title: string
  type: 'thread' | 'task' | 'section'
  status: string
  instant: string
}

export interface SectionSummary {
  id: string
  title: string
  content: string
  position: number
  isStale: boolean
  staleReason?: string
}

export interface DAGResponse {
  nodes: DAGNode[]
  edges: DAGEdge[]
  branches: BranchInfo[]
  milestones: MilestoneInfo[]
  threads: EntityNode[]
  tasks: EntityNode[]
  sections: EntityNode[]
  entityEdges: DAGEdge[]
}

export interface DecisionDetail {
  id: string
  title: string
  rationale: string
  context: string
  author: string
  instant: string
  changes: {
    entityId: string
    entityType: string
    attribute: string
    before: string | null
    after: string | null
  }[]
  relatedTasks: { id: string; title: string; status: string }[]
  sourceThread: { id: string; title: string; status: string } | null
}

export interface ThreadEntry {
  id: string
  type: string
  content: string
  author: string
  targetId?: string
  stance?: string
  isRetracted: boolean
  instant: string
}

export interface ThreadDetail {
  id: string
  title: string
  question: string
  status: string
  entries: ThreadEntry[]
}

export interface TopicSummary {
  id: string
  title: string
  description: string
  status: string
  threadIds: string[]
}

export interface TopicDetail {
  id: string
  title: string
  description: string
  status: string
  threads: { id: string; title: string; question: string; status: string }[]
}

export interface Task {
  id: string
  title: string
  description: string
  status: string
  priority: string
  assignee: string
  projectId: string
  projectName: string
  gitBranch: string
  commitCount: number
}

export interface Commit {
  id: string
  sha: string
  message: string
  author: string
  authored_at: string
  parents: string[]
  task_id?: string
  repo_id: string
  status: string
}

// --- Graph (all-branch topology) ---

export interface GraphDecision {
  id: string
  title: string
  author: string
  instant: string
  type: 'root' | 'normal' | 'merge'
  branchId: string
  branchName: string
  sourceThreadId?: string
}

export interface GraphResponse {
  decisions: GraphDecision[]
  edges: DAGEdge[]
  branches: BranchInfo[]
}
