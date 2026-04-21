package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// changeHub manages SSE client connections and broadcasts DB change events.
type changeHub struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newChangeHub() *changeHub {
	return &changeHub{
		clients: make(map[chan struct{}]struct{}),
	}
}

func (h *changeHub) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *changeHub) unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *changeHub) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// startWatcher polls the transactions table for new writes and broadcasts to SSE clients.
func (s *Server) startWatcher() {
	var lastTxID int64
	s.db.Conn().QueryRow("SELECT COALESCE(MAX(id), 0) FROM transactions").Scan(&lastTxID)

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var currentTxID int64
			err := s.db.Conn().QueryRow("SELECT COALESCE(MAX(id), 0) FROM transactions").Scan(&currentTxID)
			if err != nil {
				continue
			}
			if currentTxID > lastTxID {
				lastTxID = currentTxID
				s.hub.broadcast()
			}
		}
	}()
}

// handleSSE serves Server-Sent Events for real-time UI updates.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	// Send initial connected event
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	// Keep-alive ticker (every 30s)
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "data: {\"type\":\"change\"}\n\n")
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

