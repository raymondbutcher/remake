package uqueue

// UniqueQueue is a FIFO queue of unique strings.
type UniqueQueue struct {
	queue []string
	seen  map[string]bool
}

// NewUniqueQueue initializes a unique queue of strings.
func NewUniqueQueue() UniqueQueue {
	return UniqueQueue{
		queue: []string{},
		seen:  map[string]bool{},
	}
}

// Push adds s to the end of the queue, unless it was previously added.
func (uq *UniqueQueue) Push(s string) (added bool) {
	if uq.seen[s] {
		return false
	}
	uq.queue = append(uq.queue, s)
	uq.seen[s] = true
	return true
}

// Pop removes and returns the first string in the queue.
func (uq *UniqueQueue) Pop() (s string) {
	s = uq.queue[0]
	uq.queue = uq.queue[1:]
	return
}

// Len returns the number of items in the queue.
func (uq *UniqueQueue) Len() int {
	return len(uq.queue)
}
