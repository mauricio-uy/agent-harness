package docs

// planStates follow a plan from drafting through execution.
var planStates = []string{"draft", "awaiting-approval", "approved", "in-progress", "completed", "cancelled", "superseded"}

// Plans covers implementation plans. They share the document model of every
// other type; several states require approval of the current revision because
// execution continues under that approval.
var Plans = &Suite{Name: "plans", Types: []DocType{
	{"plan", "PLAN", "plans", "Plans", planStates, []string{"approved", "in-progress", "completed"}},
}}
