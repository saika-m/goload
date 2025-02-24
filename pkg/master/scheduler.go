package master

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
)

// Make sure our Scheduler matches the common.Scheduler interface
type scheduler struct {
	orchestrator *common.Orchestrator
	mu           sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler(orchestrator *common.Orchestrator) common.Scheduler {
	return &scheduler{
		orchestrator: orchestrator,
	}
}

// AllocateWorkers distributes virtual users across available workers
func (s *scheduler) AllocateWorkers(test *common.Test) (map[string]int, error) {
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
func (s *scheduler) getAvailableWorkers() []*common.WorkerState {
	workers := make([]*common.WorkerState, 0)
	for _, worker := range s.orchestrator.Workers {
		if s.isWorkerAvailable(worker) {
			workers = append(workers, worker)
		}
	}
	return workers
}

// isWorkerAvailable checks if a worker can accept new tests
func (s *scheduler) isWorkerAvailable(worker *common.WorkerState) bool {
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
func (s *scheduler) RebalanceWorkers(testID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	test, exists := s.orchestrator.Tests[testID]
	if !exists {
		return fmt.Errorf("test not found: %s", testID)
	}

	if test.Status.State != common.TestStateRunning {
		return fmt.Errorf("test is not running")
	}

	// Calculate new allocation
	newAllocation, err := s.AllocateWorkers(test)
	if err != nil {
		return err
	}

	// Apply the new allocation
	return s.applyAllocation(test, newAllocation)
}

func (s *scheduler) applyAllocation(test *common.Test, newAllocation map[string]int) error {
	// Get current allocation
	currentAllocation := make(map[string]int)
	for workerID, state := range test.Workers {
		currentAllocation[workerID] = state.ActiveUsers
	}

	// Calculate differences
	for workerID, newCount := range newAllocation {
		currentCount := currentAllocation[workerID]
		if newCount > currentCount {
			if err := s.addUsersToWorker(test, workerID, newCount-currentCount); err != nil {
				return err
			}
		} else if newCount < currentCount {
			if err := s.removeUsersFromWorker(test, workerID, currentCount-newCount); err != nil {
				return err
			}
		}
	}

	// Remove workers not in new allocation
	for workerID := range currentAllocation {
		if _, exists := newAllocation[workerID]; !exists {
			if err := s.removeUsersFromWorker(test, workerID, currentAllocation[workerID]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *scheduler) addUsersToWorker(test *common.Test, workerID string, count int) error {
	worker, exists := s.orchestrator.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	if worker.ActiveUsers+count > worker.Info.Capacity {
		return fmt.Errorf("worker capacity exceeded")
	}

	worker.ActiveUsers += count
	worker.Status = "running"
	worker.CurrentTestID = test.Config.TestID

	if _, exists := test.Workers[workerID]; !exists {
		test.Workers[workerID] = &common.WorkerState{
			Info:        worker.Info,
			Status:      "running",
			ActiveUsers: count,
		}
	} else {
		test.Workers[workerID].ActiveUsers += count
	}

	return nil
}

func (s *scheduler) removeUsersFromWorker(test *common.Test, workerID string, count int) error {
	worker, exists := s.orchestrator.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	worker.ActiveUsers -= count
	if worker.ActiveUsers == 0 {
		worker.Status = "idle"
		worker.CurrentTestID = ""
	}

	if testWorker, exists := test.Workers[workerID]; exists {
		testWorker.ActiveUsers -= count
		if testWorker.ActiveUsers == 0 {
			delete(test.Workers, workerID)
		}
	}

	return nil
}
