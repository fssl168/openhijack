package config

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a TOML config file for changes and calls an onChange
// callback whenever the file is modified. It uses debouncing (500 ms)
// so that multiple rapid writes fire only one callback.
type Watcher struct {
	mu       sync.Mutex
	fs       *fsnotify.Watcher
	path     string
	onChange func(*Config, error)
	running  bool
	done     chan struct{}
}

// NewWatcher starts an fsnotify-based watcher on the given TOML config file.
// On every change (debounced to 500 ms) it reloads the config and calls
// onChange with the result. Returns an error if fsnotify cannot be started.
func NewWatcher(path string, onChange func(*Config, error)) (*Watcher, error) {
	w := &Watcher{
		path:     path,
		onChange: onChange,
		done:     make(chan struct{}),
	}

	fs, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w.fs = fs

	if err := w.fs.Add(path); err != nil {
		fs.Close()
		return nil, err
	}

	w.running = true
	go w.run()
	return w, nil
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	select {
	case <-w.done:
		return nil // already closed
	default:
		close(w.done)
	}

	w.running = false
	return w.fs.Close()
}

// run is the background goroutine that processes fsnotify events with
// debouncing. Each event resets a 500 ms timer; when the timer fires
// the config is reloaded and the callback is invoked.
func (w *Watcher) run() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var timer *time.Timer
	timerFired := make(chan struct{}, 1)

	resetTimer := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(500 * time.Millisecond, func() {
			select {
			case timerFired <- struct{}{}:
			default:
			}
		})
	}

	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.fs.Events:
			if !ok {
				return
			}
			// Only react to write/remove events on our target file
			if event.Name != w.path {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Remove) == 0 {
				continue
			}
			resetTimer()
		case err, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			w.mu.Lock()
			if w.running {
				w.onChange(nil, err)
			}
			w.mu.Unlock()
		case <-ticker.C:
			// Periodic check: if file was modified but fsnotify was quiet
			// (e.g. editor overwrote file), reload anyway.
			cfg, loadErr := Load(w.path)
			w.mu.Lock()
			if w.running && w.onChange != nil {
				w.onChange(cfg, loadErr)
			}
			w.mu.Unlock()
		case <-timerFired:
			// Debounce window elapsed — reload and notify
			cfg, loadErr := Load(w.path)
			w.mu.Lock()
			if w.running && w.onChange != nil {
				w.onChange(cfg, loadErr)
			}
			w.mu.Unlock()
		}
	}
}
