package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type ChangeEvent struct {
	Path      string
	Timestamp time.Time
}

type Watcher struct {
	fsWatcher *fsnotify.Watcher
	debounce  time.Duration
	patterns  []string
}

func New(debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		fsWatcher: fsw,
		debounce:  debounce,
		patterns:  []string{".swift", ".m", ".h", ".c", ".cpp", ".metal", ".xib", ".storyboard"},
	}, nil
}

// AddRecursive adds a directory and all subdirectories.
func (w *Watcher) AddRecursive(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	return filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			base := filepath.Base(path)

			// Skip hidden, build artifacts, dependencies
			if strings.HasPrefix(base, ".") ||
				base == "DerivedData" ||
				base == "build" ||
				base == "Pods" ||
				base == "Carthage" ||
				strings.HasSuffix(path, ".xcodeproj") ||
				strings.HasSuffix(path, ".xcworkspace") {
				return filepath.SkipDir
			}

			if err := w.fsWatcher.Add(path); err != nil {
				// Log but continue - some directories may not be watchable
				return nil
			}
		}
		return nil
	})
}

// Watch returns a channel that emits debounced change events.
func (w *Watcher) Watch(ctx context.Context) <-chan ChangeEvent {
	out := make(chan ChangeEvent)

	go func() {
		defer close(out)

		var mu sync.Mutex
		var pending *time.Timer
		var lastPath string

		for {
			select {
			case <-ctx.Done():
				if pending != nil {
					pending.Stop()
				}
				return

			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return
				}

				if !w.shouldWatch(event.Name) {
					continue
				}

				// Watch for write, create, rename (atomic saves), chmod (some editors)
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Chmod) == 0 {
					continue
				}

				mu.Lock()
				lastPath = event.Name

				if pending != nil {
					pending.Stop()
				}

				pending = time.AfterFunc(w.debounce, func() {
					mu.Lock()
					p := lastPath
					mu.Unlock()

					select {
					case out <- ChangeEvent{Path: p, Timestamp: time.Now()}:
					case <-ctx.Done():
					}
				})
				mu.Unlock()

			case _, ok := <-w.fsWatcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return out
}

func (w *Watcher) shouldWatch(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, pattern := range w.patterns {
		if ext == pattern {
			return true
		}
	}
	return false
}

func (w *Watcher) Close() error {
	return w.fsWatcher.Close()
}
