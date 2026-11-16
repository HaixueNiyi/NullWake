package service

import (
	"sort"
	"sync"
	"time"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

// RingBuffer stores bounded per-service event histories strictly in memory.
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*serviceRing
}

type serviceRing struct {
	buf   []protocol.HistoryEvent
	head  int
	count int
}

// NewRingBuffer creates a thread-safe RingBuffer with capacity N events per service.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 200
	}
	return &RingBuffer{
		capacity: capacity,
		items:    make(map[string]*serviceRing),
	}
}

// Push appends an event to the ring buffer for serviceID, evicting the oldest entry if full.
func (r *RingBuffer) Push(serviceID string, ev protocol.HistoryEvent) {
	if serviceID == "" || ev.Event == "" {
		return
	}
	if ev.TS == 0 {
		ev.TS = time.Now().Unix()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ring, ok := r.items[serviceID]
	if !ok {
		ring = &serviceRing{
			buf: make([]protocol.HistoryEvent, r.capacity),
		}
		r.items[serviceID] = ring
	}

	if ring.count < r.capacity {
		idx := (ring.head + ring.count) % r.capacity
		ring.buf[idx] = ev
		ring.count++
		return
	}

	// Buffer full: overwrite oldest slot at head and advance head.
	ring.buf[ring.head] = ev
	ring.head = (ring.head + 1) % r.capacity
}

// History returns a chronological snapshot (oldest to newest) of buffered events for serviceID.
func (r *RingBuffer) History(serviceID string) []protocol.HistoryEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ring, ok := r.items[serviceID]
	if !ok || ring.count == 0 {
		return []protocol.HistoryEvent{}
	}

	out := make([]protocol.HistoryEvent, ring.count)
	for i := 0; i < ring.count; i++ {
		out[i] = ring.buf[(ring.head+i)%r.capacity]
	}
	return out
}

// ServiceIDs returns sorted service IDs that currently have at least one event in the ring buffer.
func (r *RingBuffer) ServiceIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.items))
	for id, ring := range r.items {
		if ring.count > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
