package service

import (
	"sort"
	"sync"
	"time"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

const defaultMaxServices = 64

// RingBuffer stores bounded per-service event histories strictly in memory.
type RingBuffer struct {
	mu          sync.RWMutex
	capacity    int
	maxServices int
	seq         uint64
	pinned      map[string]bool
	items       map[string]*serviceRing
}

type serviceRing struct {
	buf       []protocol.HistoryEvent
	head      int
	count     int
	lastTouch uint64
}

// NewRingBuffer creates a thread-safe RingBuffer with capacity N events per service.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 200
	}
	return &RingBuffer{
		capacity:    capacity,
		maxServices: defaultMaxServices,
		pinned:      make(map[string]bool),
		items:       make(map[string]*serviceRing),
	}
}

// Reserve pins configured service IDs so dynamic push bursts do not LRU-evict them.
func (r *RingBuffer) Reserve(serviceIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range serviceIDs {
		if id != "" {
			r.pinned[id] = true
		}
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

	r.seq++
	ring, ok := r.items[serviceID]
	if !ok {
		if len(r.items) >= r.maxServices {
			var oldestID string
			var oldestSeq uint64
			for id, candidate := range r.items {
				if r.pinned[id] {
					continue
				}
				if oldestID == "" || candidate.lastTouch < oldestSeq {
					oldestID = id
					oldestSeq = candidate.lastTouch
				}
			}
			if oldestID == "" {
				return
			}
			delete(r.items, oldestID)
		}
		ring = &serviceRing{
			buf: make([]protocol.HistoryEvent, r.capacity),
		}
		r.items[serviceID] = ring
	}
	ring.lastTouch = r.seq

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
