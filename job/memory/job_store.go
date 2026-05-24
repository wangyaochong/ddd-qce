package memory

import (
	"context"
	"fmt"
	"sync"

	jobcore "github.com/ddd-qce/core/job/core"
)

type InMemoryJobStore struct {
	jobs map[string]*jobcore.Job
	mu   sync.RWMutex
}

func NewJobStore() *InMemoryJobStore {
	return &InMemoryJobStore{
		jobs: make(map[string]*jobcore.Job),
	}
}

func (s *InMemoryJobStore) Create(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}
	s.jobs[job.ID] = job.Snapshot()
	return nil
}

func (s *InMemoryJobStore) Get(ctx context.Context, id string) (*jobcore.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job %s not found", id)
	}
	return job.Snapshot(), nil
}

func (s *InMemoryJobStore) Update(ctx context.Context, job *jobcore.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID]; !exists {
		return fmt.Errorf("job %s not found", job.ID)
	}
	s.jobs[job.ID] = job.Snapshot()
	return nil
}

func (s *InMemoryJobStore) List(ctx context.Context, status jobcore.JobStatus) ([]*jobcore.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*jobcore.Job
	for _, job := range s.jobs {
		if job.Status == status {
			result = append(result, job.Snapshot())
		}
	}
	return result, nil
}

func (s *InMemoryJobStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[id]; !exists {
		return fmt.Errorf("job %s not found", id)
	}
	delete(s.jobs, id)
	return nil
}
