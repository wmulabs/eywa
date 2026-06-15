package entities

// PlanStatus is the lifecycle state of a single plan item maintained by the agent during reasoning.
type PlanStatus string

const (
	PlanPending    PlanStatus = "pending"
	PlanInProgress PlanStatus = "in_progress"
	PlanDone       PlanStatus = "done"
	PlanAbandoned  PlanStatus = "abandoned"
)

// PlanItem is one entry in the agent's turn-scoped plan/scratchpad.
type PlanItem struct {
	Title  string     `bson:"title" json:"title"`
	Status PlanStatus `bson:"status" json:"status"`
}
