package store

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// DeployRecord holds the state and logs for one deployment execution.
type DeployRecord struct {
	ID        string
	Project   string
	Service   string
	Image     string
	Status    string
	StartTime time.Time
	EndTime   time.Time // zero value until Complete() is called
	Logs      []string

	mu sync.Mutex // guards Logs, Status, EndTime
}

func (r *DeployRecord) AppendLog(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Logs = append(r.Logs, line)
}

func (r *DeployRecord) Complete(success bool, endTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if success {
		r.Status = StatusSuccess
	} else {
		r.Status = StatusFailed
	}
	r.EndTime = endTime
}

// Snapshot returns a copy safe to serialize without holding the lock.
func (r *DeployRecord) Snapshot() DeployRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	logs := make([]string, len(r.Logs))
	copy(logs, r.Logs)
	return DeployRecord{
		ID:        r.ID,
		Project:   r.Project,
		Service:   r.Service,
		Image:     r.Image,
		Status:    r.Status,
		StartTime: r.StartTime,
		EndTime:   r.EndTime,
		Logs:      logs,
	}
}

// Store holds all deployment records in memory.
type Store struct {
	mu      sync.RWMutex
	records []*DeployRecord
	index   map[string]*DeployRecord
}

func New() *Store {
	return &Store{index: make(map[string]*DeployRecord)}
}

func (s *Store) Add(r *DeployRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	s.index[r.ID] = r
}

// Get returns the live pointer for a record by ID (nil if not found).
func (s *Store) Get(id string) *DeployRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index[id]
}

// List returns snapshots of all records, newest first.
func (s *Store) List() []DeployRecord {
	s.mu.RLock()
	ptrs := make([]*DeployRecord, len(s.records))
	copy(ptrs, s.records)
	s.mu.RUnlock()

	result := make([]DeployRecord, len(ptrs))
	for i, p := range ptrs {
		result[len(ptrs)-1-i] = p.Snapshot()
	}
	return result
}

// GenerateID returns a random UUID v4 string.
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
