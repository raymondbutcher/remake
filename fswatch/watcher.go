package fswatch

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
)

// SharedWatcher is a wrapper for fsnotify.Watcher that allows for
// multiple recipients (clients) to receive filesystem notifications.
type SharedWatcher struct {
	Errors   chan error
	debounce time.Duration
	watcher  *fsnotify.Watcher
	clients  []*Client
	mutex    sync.Mutex
	notify   <-chan time.Time
	changed  bool
	closed   chan struct{}
}

// NewSharedWatcher initializes a shared watcher.
func NewSharedWatcher(debounce time.Duration) *SharedWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("NewSharedWatcher: fsnotify.Watcher error: %s", err)
	}
	sw := SharedWatcher{
		Errors:   watcher.Errors,
		debounce: debounce,
		watcher:  watcher,
		notify:   make(<-chan time.Time),
		closed:   make(chan struct{}),
	}
	return &sw
}

// Add starts watching the named file or directory.
func (sw *SharedWatcher) Add(name string) error {
	return sw.watcher.Add(name)
}

// AddDir starts watched the named directory,
// or the file's directory if given the name of a file.
func (sw *SharedWatcher) AddDir(name string) error {
	if fi, err := os.Stat(name); err == nil && !fi.IsDir() {
		name = filepath.Dir(name)
	}
	return sw.Add(name)
}

// NewClient creates a client for the watcher, to receive notifications.
func (sw *SharedWatcher) NewClient() *Client {
	sw.mutex.Lock()
	c := NewClient(sw)
	sw.clients = append(sw.clients, c)
	sw.mutex.Unlock()
	return c
}

// Close removes all watches and closes the events channel.
func (sw *SharedWatcher) Close() error {
	sw.closed <- struct{}{}
	if err := sw.watcher.Close(); err != nil {
		return err
	}
	sw.mutex.Lock()
	for _, client := range sw.clients {
		client.close()
	}
	sw.mutex.Unlock()
	return nil
}

// Remove stops watching the named file or directory.
func (sw *SharedWatcher) Remove(name string) error {
	return sw.watcher.Remove(name)
}

// Start receiving events from the filesystem, and notify clients about events
// after the debounce period.
func (sw *SharedWatcher) Start() {
	go func() {
		for {
			select {
			case event := <-sw.watcher.Events:
				if strings.HasPrefix(filepath.Base(event.Name), ".") {
					// Ignore all dot files/dirs as they probably won't make
					// any difference. This is mainly to ignore version
					// control directories.
					continue
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					// Watch all files/directories that are encountered.
					// This is a quasi-recursive solution, adding all
					// encountered directories rather than files, but
					// not actually recursively.
					sw.AddDir(event.Name)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					// Stop watching removed paths. Ignore errors on purpose
					// as they might not have been added in the first place.
					sw.Remove(event.Name)
				} else if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					// Ignore chmod events because they include when access
					// times are modified. It would be nice ignore only that
					// scenario but it doesn't have that level of detail.
					continue
				}
				// Send to the sw.notify channel, but with debouncing.
				debounceNotify(sw)
			case <-sw.notify:
				sw.notifyClients()
			case <-sw.closed:
				return
			}
		}
	}()
}

func (sw *SharedWatcher) notifyClients() {
	sw.mutex.Lock()
	for _, client := range sw.clients {
		client.Notify()
	}
	sw.mutex.Unlock()
}

// debounceNotify sets a timer channel to notify clients about filesystem
// events. It replaces the previous channel, which has the effect of
// grouping multiple debounceNotify calls into a single notification.
var debounceNotify = func(sw *SharedWatcher) {
	sw.notify = time.After(sw.debounce)
}

// A Client receives notifications from a watcher.
type Client struct {
	Watcher  *SharedWatcher
	C        chan bool
	changed  bool
	mutex    sync.Mutex
	notified chan bool
	closed   chan bool
}

// NewClient creates a watcher client.
func NewClient(sw *SharedWatcher) *Client {
	c := Client{
		Watcher:  sw,
		C:        make(chan bool),
		notified: make(chan bool, 1),
		closed:   make(chan bool, 1),
	}
	go func() {
		for {
			select {
			case <-c.notified:
				c.mutex.Lock()
				c.changed = false
				c.mutex.Unlock()
				c.C <- true
			case <-c.closed:
				return
			}
		}
	}()
	return &c
}

func (c *Client) close() {
	c.closed <- true
}

// Notify the client that a filesystem event has occurred. Only 1 notification
// is buffered, subsequent calls will be silently ignored until the last
// notification is received via the C channel.
func (c *Client) Notify() {
	c.mutex.Lock()
	if !c.changed {
		c.changed = true
		c.notified <- true
	}
	c.mutex.Unlock()
}
