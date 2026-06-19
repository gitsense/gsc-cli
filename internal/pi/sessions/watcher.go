/**
 * Component: Pi Sessions Continuous Watcher
 * Block-UUID: 617d28e4-2d83-43f3-8a2c-700d8cf9fdf3
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Runs foreground continuous Pi session reconciliation using recursive filesystem events, debounce coalescing, and periodic scans.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	defaultWatchDebounceInterval  = 200 * time.Millisecond
	defaultReconcileInterval      = 30 * time.Second
	defaultDirectoryRefreshPeriod = 5 * time.Second
)

type WatchOptions struct {
	SessionsDir       string
	DBPath            string
	DebounceInterval  time.Duration
	ReconcileInterval time.Duration
	PollInterval      time.Duration
}

type watchEventSource interface {
	Events() <-chan string
	Errors() <-chan error
	Close() error
}

func Watch(ctx context.Context, options WatchOptions) error {
	if options.SessionsDir == "" {
		return fmt.Errorf("sessions dir is required")
	}
	if options.DBPath == "" {
		return fmt.Errorf("db path is required")
	}
	sessionsDir, err := filepath.Abs(options.SessionsDir)
	if err != nil {
		return err
	}
	if options.DebounceInterval <= 0 {
		options.DebounceInterval = defaultWatchDebounceInterval
	}
	if options.ReconcileInterval <= 0 {
		options.ReconcileInterval = defaultReconcileInterval
	}
	if options.PollInterval <= 0 {
		options.PollInterval = defaultDirectoryRefreshPeriod
	}
	source, err := newFSNotifyEventSource(sessionsDir, options.PollInterval)
	if err != nil {
		return err
	}
	defer source.Close()
	reconcile := func(reconcileCtx context.Context) error {
		_, err := Sync(reconcileCtx, SyncOptions{SessionsDir: sessionsDir, DBPath: options.DBPath})
		return err
	}
	return runWatchLoop(ctx, options.DebounceInterval, options.ReconcileInterval, source, reconcile)
}

func runWatchLoop(ctx context.Context, debounceInterval time.Duration, reconcileInterval time.Duration, source watchEventSource, reconcile func(context.Context) error) error {
	if err := reconcile(ctx); err != nil {
		return fmt.Errorf("initial Pi sessions reconciliation: %w", err)
	}
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	pending := make(map[string]struct{})
	var debounceTimer *time.Timer
	var debounce <-chan time.Time
	stopDebounce := func() {
		if debounceTimer != nil && !debounceTimer.Stop() {
			select {
			case <-debounceTimer.C:
			default:
			}
		}
		debounce = nil
	}
	reconcilePending := func() error {
		stopDebounce()
		if err := reconcile(ctx); err != nil {
			return err
		}
		clear(pending)
		return nil
	}

	events := source.Events()
	errors := source.Errors()
	for {
		select {
		case <-ctx.Done():
			stopDebounce()
			return nil
		case path, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			pending[path] = struct{}{}
			if debounceTimer == nil {
				debounceTimer = time.NewTimer(debounceInterval)
			} else {
				stopDebounce()
				debounceTimer.Reset(debounceInterval)
			}
			debounce = debounceTimer.C
		case <-debounce:
			if len(pending) > 0 {
				if err := reconcilePending(); err != nil {
					return fmt.Errorf("event-driven Pi sessions reconciliation: %w", err)
				}
			}
		case <-ticker.C:
			if err := reconcilePending(); err != nil {
				return fmt.Errorf("periodic Pi sessions reconciliation: %w", err)
			}
		case _, ok := <-errors:
			if !ok {
				errors = nil
				continue
			}
			if err := reconcilePending(); err != nil {
				return fmt.Errorf("filesystem watcher recovery reconciliation: %w", err)
			}
		}
	}
}

type fsnotifyEventSource struct {
	watcher *fsnotify.Watcher
	root    string
	events  chan string
	errors  chan error
	done    chan struct{}
	cancel  context.CancelFunc
	once    sync.Once
}

func newFSNotifyEventSource(root string, refreshInterval time.Duration) (*fsnotifyEventSource, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create Pi sessions filesystem watcher: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	source := &fsnotifyEventSource{
		watcher: watcher,
		root:    root,
		events:  make(chan string, 256),
		errors:  make(chan error, 16),
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	if err := source.addDirectories(); err != nil {
		cancel()
		watcher.Close()
		return nil, err
	}
	go source.run(ctx, refreshInterval)
	return source, nil
}

func (source *fsnotifyEventSource) Events() <-chan string { return source.events }
func (source *fsnotifyEventSource) Errors() <-chan error  { return source.errors }

func (source *fsnotifyEventSource) Close() error {
	var closeErr error
	source.once.Do(func() {
		source.cancel()
		closeErr = source.watcher.Close()
		<-source.done
	})
	return closeErr
}

func (source *fsnotifyEventSource) run(ctx context.Context, refreshInterval time.Duration) {
	defer close(source.done)
	defer close(source.events)
	defer close(source.errors)
	refresh := time.NewTicker(refreshInterval)
	defer refresh.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-source.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := source.addDirectories(); err != nil {
						source.emitError(err)
					}
				}
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				source.emitEvent(event.Name)
			}
		case err, ok := <-source.watcher.Errors:
			if !ok {
				return
			}
			source.emitError(err)
		case <-refresh.C:
			if err := source.addDirectories(); err != nil {
				source.emitError(err)
			}
		}
	}
}

func (source *fsnotifyEventSource) addDirectories() error {
	return filepath.WalkDir(source.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		for _, watched := range source.watcher.WatchList() {
			if watched == path {
				return nil
			}
		}
		if err := source.watcher.Add(path); err != nil {
			return fmt.Errorf("watch Pi sessions directory %s: %w", path, err)
		}
		return nil
	})
}

func (source *fsnotifyEventSource) emitEvent(path string) {
	select {
	case source.events <- path:
	default:
		source.emitError(fmt.Errorf("Pi sessions filesystem event queue overflow"))
	}
}

func (source *fsnotifyEventSource) emitError(err error) {
	select {
	case source.errors <- err:
	default:
	}
}
