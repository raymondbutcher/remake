package watcher

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
)

// SharedWatcher is a wrapper for fsnotify.Watcher that supports multiple
// recipients (clients) of filesystem events.
type SharedWatcher struct {
	Errors  chan error
	watcher *fsnotify.Watcher
	clients []*Client
	mutex   sync.Mutex
}

// NewSharedWatcher creates a Watcher and starts a goroutine for handling
// filesystem events and errors.
func NewSharedWatcher(debounce time.Duration) *SharedWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	sw := SharedWatcher{
		Errors:  watcher.Errors,
		watcher: watcher,
	}

	go func() {
		changed := false
		check := time.After(debounce)
		for {
			select {
			case event := <-watcher.Events:
				if strings.HasPrefix(filepath.Base(event.Name), ".") {
					continue
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					// Watch all files/directories.
					sw.Add(event.Name)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					// Remove this if it is being watched.
					// Ignore errors on purpose.
					sw.Remove(event.Name)
				} else if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					// Ignore chmod events because they include when access
					// times are modified. It would be nice ignore that
					// scenario but it doesn't have that level of detail.
					continue
				}
				changed = true
				check = time.After(debounce)
			case <-check:
				if changed {
					changed = false
					sw.mutex.Lock()
					for _, client := range sw.clients {
						client.notify()
					}
					sw.mutex.Unlock()
				}
			}
		}
	}()

	return &sw
}

// Add starts watching the named file or directory.
func (sw *SharedWatcher) Add(name string) error {
	//log.Println("Watcher.Add", name)
	return sw.watcher.Add(name)
}

// Client creates a client for the watcher, to receive events.
func (sw *SharedWatcher) Client() *Client {
	sw.mutex.Lock()
	c := NewClient(sw)
	sw.clients = append(sw.clients, c)
	sw.mutex.Unlock()
	return c
}

// Close removes all watches and closes the events channel.
func (sw *SharedWatcher) Close() error {
	return sw.watcher.Close()
}

// Remove stops watching the named file or directory.
func (sw *SharedWatcher) Remove(name string) error {
	return sw.watcher.Remove(name)
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
