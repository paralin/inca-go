package segment

import (
	"container/heap"
	"sync"
)

// SegmentQueue queues rewinding segments.
type SegmentQueue struct {
	mtx      sync.Mutex
	pq       segmentHeightPq
	notifyCh chan<- *Segment
}

// NewSegmentQueue builds a new segment queue.
func NewSegmentQueue(notifyCh chan<- *Segment) *SegmentQueue {
	sq := &SegmentQueue{notifyCh: notifyCh}
	heap.Init(&sq.pq)
	return sq
}

// Push adds a segment to the queue.
func (sq *SegmentQueue) Push(seg *Segment) {
	sq.mtx.Lock()
	defer sq.mtx.Unlock()

	for _, s := range sq.pq {
		if s == seg {
			return
		}
	}

	heap.Push(&sq.pq, seg)
	select {
	case sq.notifyCh <- seg:
	default:
	}
}

// Remove removes a segment from the queue.
func (sq *SegmentQueue) Remove(seg *Segment) {
	sq.mtx.Lock()
	defer sq.mtx.Unlock()

	for i, segi := range sq.pq {
		if seg == segi {
			heap.Remove(&sq.pq, i)
			break
		}
	}
}

// Pop pops a segment from the queue.
func (sq *SegmentQueue) Pop() *Segment {
	return heap.Pop(&sq.pq).(*Segment)
}

// segmentHeightPq sorts Segments by their height, lower height first.
type segmentHeightPq []*Segment

func (pq segmentHeightPq) Len() int { return len(pq) }

func (pq segmentHeightPq) Less(i, j int) bool {
	return pq[i].state.GetTailBlockRound().GetHeight() < pq[i].state.GetTailBlockRound().GetHeight()
}

func (pq segmentHeightPq) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *segmentHeightPq) Push(x interface{}) {
	item := x.(*Segment)
	*pq = append(*pq, item)
}

func (pq *segmentHeightPq) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}
