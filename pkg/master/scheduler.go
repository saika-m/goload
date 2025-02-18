package master

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
)

// Scheduler handles worker allocation and load distribution
type Scheduler struct {
	orchestrator *Orchestrator
	mu           sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler(orchestrator *Orchestrator) *Scheduler {
	return &Scheduler{
		orchestrator: orchestrator,
	}
}

// WorkerAllocation represents the allocation of virtual users to a worker
type WorkerAllocation struct {
	WorkerID     string
	VirtualUsers int
	Location     string
}

// AllocateWorkers distributes virtual users across available workers
func (s *Scheduler) AllocateWorkers(test *Test) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get available workers
	availableWorkers := s.getAvailableWorkers()
	if len(availableWorkers) == 0 {
		return nil, fmt.Errorf("no workers available")
	}

	// Calculate total capacity
	totalCapacity := 0
	for _, worker := range availableWorkers {
		totalCapacity += worker.Info.Capacity
	}

	if totalCapacity < test.Config.VirtualUsers {
		return nil, fmt.Errorf("insufficient worker capacity: need %d, have %d",
			test.Config.VirtualUsers, totalCapacity)
	}

	// Sort workers by capacity (descending)
	sort.Slice(availableWorkers, func(i, j int) bool {
		return availableWorkers[i].Info.Capacity > availableWorkers[j].Info.Capacity
	})

	// Distribute load
	allocation := make(map[string]int)
	remainingUsers := test.Config.VirtualUsers

	// First pass: distribute users proportionally to capacity
	for _, worker := range availableWorkers {
		// Calculate worker's share based on its capacity
		share := (worker.Info.Capacity * remainingUsers) / totalCapacity
		if share > 0 {
			allocation[worker.Info.ID] = share
			remainingUsers -= share
			totalCapacity -= worker.Info.Capacity
		}
	}

	// Second pass: distribute remaining users
	if remainingUsers > 0 {
		for _, worker := range availableWorkers {
			if worker.Info.Capacity > allocation[worker.Info.ID] {
				allocation[worker.Info.ID]++
				remainingUsers--
				if remainingUsers == 0 {
					break
				}
			}
		}
	}

	return allocation, nil
}

// getAvailableWorkers returns a list of workers that can accept new tests
func (s *Scheduler) getAvailableWorkers() []*WorkerState {
	s.orchestrator.mu.RLock()
	defer s.orchestrator.mu.RUnlock()

	available := make([]*WorkerState, 0)
	for _, worker := range s.orchestrator.workers {
		if s.isWorkerAvailable(worker) {
			available = append(available, worker)
		}
	}
	return available
}

// isWorkerAvailable checks if a worker can accept new tests
func (s *Scheduler) isWorkerAvailable(worker *WorkerState) bool {
	// Check if worker is idle
	if worker.Status != "idle" && worker.Status != "ready" {
		return false
	}

	// Check if worker is healthy (recent heartbeat)
	if time.Since(worker.LastHeartbeat) > 30*time.Second {
		return false
	}

	// Check if worker has capacity
	if worker.ActiveUsers >= worker.Info.Capacity {
		return false
	}

	// Check resource utilization
	if worker.Info.Resources.CPUUsage > 80 || worker.Info.Resources.MemoryUsage > 80 {
		return false
	}

	return true
}

// RebalanceWorkers redistributes load when workers join or leave
func (s *Scheduler) RebalanceWorkers(testID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	test, err := s.orchestrator.GetTest(testID)
	if err != nil {
		return err
	}

	if test.Status.State != common.TestStateRunning {
		return fmt.Errorf("test is not running")
	}

	// Get current allocation
	currentAllocation := make(map[string]int)
	for workerID, state := range test.Workers {
		currentAllocation[workerID] = state.ActiveUsers
	}

	// Calculate new allocation
	newAllocation, err := s.AllocateWorkers(test)
	if err != nil {
		return err
	}

	// Calculate differences
	toAdd := make(map[string]int)
	toRemove := make(map[string]int)

	for workerID, newCount := range newAllocation {
		if currentCount, exists := currentAllocation[workerID]; exists {
			if newCount > currentCount {
				toAdd[workerID] = newCount - currentCount
			} else if newCount < currentCount {
				toRemove[workerID] = currentCount - newCount
			}
		} else {
			toAdd[workerID] = newCount
		}
	}

	for workerID, currentCount := range currentAllocation {
		if _, exists := newAllocation[workerID]; !exists {
			toRemove[workerID] = currentCount
		}
	}

	// Apply changes
	for workerID, count := range toRemove {
		if err := s.removeUsersFromWorker(test, workerID, count); err != nil {
			return err
		}
	}

	for workerID, count := range toAdd {
		if err := s.addUsersToWorker(test, workerID, count); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scheduler) removeUsersFromWorker(test *Test, workerID string, count int) error {
	worker, exists := s.orchestrator.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Update worker state
	worker.ActiveUsers -= count
	if worker.ActiveUsers == 0 {
		worker.Status = "idle"
		worker.CurrentTestID = ""
	}

	// Update test state
	if testWorker, exists := test.Workers[workerID]; exists {
		testWorker.ActiveUsers -= count
		if testWorker.ActiveUsers == 0 {
			delete(test.Workers, workerID)
		}
	}

	// TODO: Send gRPC command to worker to stop users

	return nil
}

func (s *Scheduler) addUsersToWorker(test *Test, workerID string, count int) error {
	worker, exists := s.orchestrator.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Check capacity
	if worker.ActiveUsers+count > worker.Info.Capacity {
		return fmt.Errorf("worker capacity exceeded")
	}

	// Update worker state
	worker.ActiveUsers += count
	worker.Status = "running"
	worker.CurrentTestID = test.Config.TestID

	// Update test state
	if _, exists := test.Workers[workerID]; !exists {
		test.Workers[workerID] = &WorkerState{
			Info:        worker.Info,
			ActiveUsers: count,
			Status:      "running",
		}
	} else {
		test.Workers[workerID].ActiveUsers += count
	}

	// TODO: Send gRPC command to worker to start users

	return nil
}

// HandleWorkerFailure manages redistribution of load when a worker fails
func (s *Scheduler) HandleWorkerFailure(workerID string) error {
	s.orchestrator.mu.Lock()
	defer s.orchestrator.mu.Unlock()

	worker, exists := s.orchestrator.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if worker.CurrentTestID == "" {
		return nil
	}

	// Get the affected test
	test, err := s.orchestrator.GetTest(worker.CurrentTestID)
	if err != nil {
		return err
	}

	// Remove the failed worker
	delete(s.orchestrator.workers, workerID)
	delete(test.Workers, workerID)

	// Attempt to rebalance the test
	return s.RebalanceWorkers(test.Config.TestID)
}
