package store

import "secsim/design/backend/internal/model"

const snapshotSubscriberBuffer = 16

func (s *Store) SubscribeSnapshots() (<-chan model.Snapshot, model.Snapshot, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	channel := make(chan model.Snapshot, snapshotSubscriberBuffer)
	s.subscribers[channel] = struct{}{}

	return channel, s.snapshotLocked(), func() {
		s.unsubscribeSnapshot(channel)
	}
}

func (s *Store) unsubscribeSnapshot(channel chan model.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.subscribers[channel]; !ok {
		return
	}

	delete(s.subscribers, channel)
	close(channel)
}

func (s *Store) publishSnapshotLocked(snapshot model.Snapshot) {
	for channel := range s.subscribers {
		select {
		case channel <- model.CloneSnapshot(snapshot):
		default:
		}
	}
}
