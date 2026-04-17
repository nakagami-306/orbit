package eavt

// Attribute constants for all entity types, namespaced.

// Project attributes
const (
	AttrProjectName        = "project/name"
	AttrProjectDescription = "project/description"
	AttrProjectStatus      = "project/status"
	AttrProjectTags        = "project/tags"
)

// Section attributes
const (
	AttrSectionTitle     = "section/title"
	AttrSectionContent   = "section/content"
	AttrSectionPosition  = "section/position"
	AttrSectionProjectID = "section/project_id"
	AttrSectionRef       = "section/ref" // Reference link: value is Ref to target section
)

// Decision attributes
const (
	AttrDecisionTitle        = "decision/title"
	AttrDecisionRationale    = "decision/rationale"
	AttrDecisionContext      = "decision/context"
	AttrDecisionParents      = "decision/parents"
	AttrDecisionSourceThread = "decision/source_thread"
	AttrDecisionAuthor       = "decision/author"
	AttrDecisionProjectID    = "decision/project_id"
)

// Branch attributes
const (
	AttrBranchName         = "branch/name"
	AttrBranchProjectID    = "branch/project_id"
	AttrBranchHeadDecision = "branch/head_decision"
	AttrBranchStatus       = "branch/status"
	AttrBranchIsMain       = "branch/is_main"
)

// Thread attributes
const (
	AttrThreadTitle           = "thread/title"
	AttrThreadQuestion        = "thread/question"
	AttrThreadStatus          = "thread/status"
	AttrThreadProjectID       = "thread/project_id"
	AttrThreadOutcomeDecision = "thread/outcome_decision"
)

// Entry attributes
const (
	AttrEntryThreadID    = "entry/thread_id"
	AttrEntryType        = "entry/type"
	AttrEntryContent     = "entry/content"
	AttrEntryTargetID    = "entry/target_id"
	AttrEntryStance      = "entry/stance"
	AttrEntryAuthor      = "entry/author"
	AttrEntryIsRetracted = "entry/is_retracted"
)

// Task attributes
const (
	AttrTaskTitle       = "task/title"
	AttrTaskDescription = "task/description"
	AttrTaskStatus      = "task/status"
	AttrTaskPriority    = "task/priority"
	AttrTaskAssignee    = "task/assignee"
	AttrTaskProjectID   = "task/project_id"
	AttrTaskSourceType  = "task/source_type"
	AttrTaskSourceID    = "task/source_id"
	AttrTaskDueDate     = "task/due_date"
	AttrTaskTags        = "task/tags"
)

// Milestone attributes
const (
	AttrMilestoneTitle       = "milestone/title"
	AttrMilestoneDescription = "milestone/description"
	AttrMilestoneProjectID   = "milestone/project_id"
	AttrMilestoneDecisionID  = "milestone/decision_id"
)

// Topic attributes
const (
	AttrTopicTitle           = "topic/title"
	AttrTopicDescription     = "topic/description"
	AttrTopicProjectID       = "topic/project_id"
	AttrTopicStatus          = "topic/status"
	AttrTopicOutcomeDecision = "topic/outcome_decision"
)

// TopicThread link attributes
const (
	AttrTopicThreadTopicID  = "topic_thread/topic_id"
	AttrTopicThreadThreadID = "topic_thread/thread_id"
)

// Decision additional attribute
const (
	AttrDecisionSourceTopic = "decision/source_topic"
)
