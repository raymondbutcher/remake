package fswatch

import (
	"fmt"
	"testing"
	"time"

	"gopkg.in/fsnotify.v1"
)

func TestClientNotify(t *testing.T) {
	// Create a watcher, but mock the debounceNotify functionionality,
	// and get a notify channel that can be used to manually trigger
	// client notifications.
	sw := NewSharedWatcher(1 * time.Millisecond)
	notify, restoreDebounceNotify := mockDebounceNotify(sw)
	defer restoreDebounceNotify()

	// Start the watcher and create 2 clients to receive notifications.
	sw.Start()
	defer sw.Close()
	c1 := sw.NewClient()
	c2 := sw.NewClient()

	// Add 3 fake filesystem events.
	for i := range []int{1, 2, 3} {
		sw.watcher.Events <- fsnotify.Event{
			Name: fmt.Sprintf("fake%d.txt", i),
			Op:   fsnotify.Create,
		}
	}

	// Trigger client notifications that would normally occur
	// after the debounce delay.
	notify <- time.Time{}

	if !clientHasOne(c1) {
		t.Fatalf("client 1 did not get exactly one event")
	}
	if !clientHasOne(c2) {
		t.Fatalf("client 2 did not get exactly one event")
	}

	// Add another fake event and trigger notifications.
	sw.watcher.Events <- fsnotify.Event{
		Name: "fake.txt",
		Op:   fsnotify.Create,
	}
	notify <- time.Time{}

	if !clientHasOne(c1) {
		t.Fatalf("client 1 did not get exactly one event")
	}
	if !clientHasOne(c2) {
		t.Fatalf("client 2 did not get exactly one event")
	}
}

// mockDebounceNotify disables debounceNotify, the debounce-based client
// notification function, and returns a channel that can be used to manually
// trigger notifications.
func mockDebounceNotify(
	sw *SharedWatcher,
) (
	notify chan time.Time,
	restore func(),
) {
	notify = make(chan time.Time)
	sw.notify = notify

	old := debounceNotify
	debounceNotify = func(sw *SharedWatcher) {}
	restore = func() {
		debounceNotify = old
	}

	return
}

// clientHasOne checks that a client has exactly one value in its channel.
func clientHasOne(c *Client) bool {
	// Client.Notify() purposefully makes it asynchronous for the watcher,
	// and it will ignore notifications when a previous notification has not
	// been consumed yet. So the checking here is not perfect.
	for {
		select {
		case <-c.C:
			// Received 1. Now ensure that there are 0 left. This is not
			// allowing for more to come in slowly, which is not ideal,
			// but I don't want the test to wait around for something
			// that shouldn't ever happen.
			select {
			case <-c.C:
				return false
			default:
				return true
			}
		case <-time.After(1 * time.Second):
			// Using a timeout is not ideal.
			return false
		}
	}
}
