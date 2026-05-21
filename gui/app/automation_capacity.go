package app

import "strings"

// AutomationWorkerBudget is the shared daemon worker budget surface used by
// coordinator-owned automation that starts runs through AgentService instead of
// the daemon queue. The production daemon implements it; tests use fakes.
type AutomationWorkerBudget interface {
	MaxWorkers() int
	ActiveTaskIDs() []string
}

type automationWorkerReservoir interface {
	TryReserveAutomationRun(taskID string) bool
	ReleaseAutomationRun(taskID string)
}

type activeTaskLister interface {
	ActiveTaskIDs() []string
}

func remainingAutomationWorkerCapacity(budget AutomationWorkerBudget, agent activeTaskLister, settings *SettingsService) int {
	maxWorkers := 1
	active := map[string]struct{}{}

	if budget != nil {
		if max := budget.MaxWorkers(); max > 0 {
			maxWorkers = max
		}
		for _, id := range budget.ActiveTaskIDs() {
			if strings.TrimSpace(id) != "" {
				active[id] = struct{}{}
			}
		}
	} else if settings != nil {
		if max := settings.GetMaxWorkers(); max > 0 {
			maxWorkers = max
		}
	}

	if agent != nil {
		for _, id := range agent.ActiveTaskIDs() {
			if strings.TrimSpace(id) != "" {
				active[id] = struct{}{}
			}
		}
	}

	remaining := maxWorkers - len(active)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func reserveAutomationWorker(budget AutomationWorkerBudget, taskID string) (func(), bool) {
	reservoir, ok := budget.(automationWorkerReservoir)
	if !ok {
		return func() {}, true
	}
	if !reservoir.TryReserveAutomationRun(taskID) {
		return nil, false
	}
	return func() { reservoir.ReleaseAutomationRun(taskID) }, true
}
