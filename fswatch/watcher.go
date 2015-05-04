package fswatch

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
)

// SharedWatcher is a wrapper for fsnotify.Watcher that supports multiple
// recipients (clients) of filesystem events.
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

// NewSharedWatcher creates a Watcher and starts a goroutine for handling
// filesystem events and errors.
func NewSharedWatcher(debounce time.Duration) *SharedWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
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
	//log.Println("Watcher.Add", name)
	return sw.watcher.Add(name)
}

// NewClient creates a client for the watcher, to receive events.
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
	return sw.watcher.Close()
}

// Remove stops watching the named file or directory.
func (sw *SharedWatcher) Remove(name string) error {
	return sw.watcher.Remove(name)
}

// Start receiving events from the filesystem.
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
					if fi, err := os.Stat(event.Name); err == nil && !fi.IsDir() {
						sw.Add(filepath.Dir(event.Name))
					} else {
						sw.Add(event.Name)
					}
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					// Stop watching removed paths. Ignore errors on purpose
					// as they might not have been added in the first place.
					sw.Remove(event.Name)
				} else if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					// Ignore chmod events because they include when access
					// times are modified. It would be nice ignore that
					// scenario but it doesn't have that level of detail.
					continue
				}
				debounceNotify(sw)
			case <-sw.notify:
				sw.mutex.Lock()
				for _, client := range sw.clients {
					client.notify()
				}
				sw.mutex.Unlock()
			case <-sw.closed:
				return
			}
		}
	}()
}

// debounceNotify sets a timer channel to notify clients about filesystem
// events. It replaces the previous channel, to group multiple filesystem
// events into a single client notification.
var debounceNotify = func(sw *SharedWatcher) {
	sw.notify = time.After(sw.debounce)
}

// Client of a watcher, to receive events.
type Client struct {
	Watcher  *SharedWatcher
	Events   chan bool
	changed  bool
	mutex    sync.Mutex
	notified chan bool
}

// NewClient creates a watcher client.
func NewClient(sw *SharedWatcher) *Client {
	c := Client{
		Watcher:  sw,
		Events:   make(chan bool),
		notified: make(chan bool, 1),
	}
	go func() {
		for {
			<-c.notified
			c.mutex.Lock()
			c.changed = false
			c.mutex.Unlock()
			c.Events <- true
		}
	}()
	return &c
}

func (c *Client) notify() {
	c.mutex.Lock()
	if !c.changed {
		c.changed = true
		c.notified <- true
	}
	c.mutex.Unlock()
}
