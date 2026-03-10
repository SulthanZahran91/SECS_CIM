package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"secsim/design/backend/internal/model"
)

const eventsKeepAliveInterval = 25 * time.Second

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	updates, initial, unsubscribe := h.store.SubscribeSnapshots()
	defer unsubscribe()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	if err := writeSnapshotEvent(w, initial); err != nil {
		return
	}
	flusher.Flush()

	keepAlive := time.NewTicker(eventsKeepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSnapshotEvent(w, snapshot); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSnapshotEvent(w http.ResponseWriter, snapshot model.Snapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}
