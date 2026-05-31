package http

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

const livereloadScript = `<script>(function(){var es=new EventSource('/livereload');es.onmessage=function(e){if(e.data==='reload')location.reload();};})();</script>`

type LiveReloadServer struct {
	mu        sync.Mutex
	clients   map[chan struct{}]struct{}
	watcher   *fsnotify.Watcher
	stale     atomic.Bool
	closeOnce sync.Once
	closed    chan struct{}
}

func NewLiveReloadServer(templateDir string) (*LiveReloadServer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	if err := watcher.Add(templateDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watch template dir: %w", err)
	}

	s := &LiveReloadServer{
		clients: make(map[chan struct{}]struct{}),
		watcher: watcher,
		closed:  make(chan struct{}),
	}

	go s.watch()
	log.Printf("[LiveReload] Watching %s for changes", templateDir)
	return s, nil
}

func (s *LiveReloadServer) watch() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				if filepath.Ext(event.Name) == ".html" {
					s.stale.Store(true)
					s.broadcast()
				}
			}
		case _, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
		case <-s.closed:
			return
		}
	}
}

func (s *LiveReloadServer) broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *LiveReloadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-ch:
			fmt.Fprintf(w, "data: reload\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-s.closed:
			return
		}
	}
}

func (s *LiveReloadServer) IsStale() bool {
	return s.stale.Load()
}

func (s *LiveReloadServer) ClearStale() {
	s.stale.Store(false)
}

func (s *LiveReloadServer) Close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.watcher.Close()
	})
}
