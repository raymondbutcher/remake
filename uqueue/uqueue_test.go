package uqueue

import (
	"strings"
	"testing"
)

func TestUniqueQueue(t *testing.T) {

	uq := NewUniqueQueue()

	uq.Push("a")
	uq.Push("b")
	uq.Push("a")
	uq.Push("c")
	uq.Push("a")

	if uq.Len() != 3 {
		t.Errorf("Expected len=3 got len=%d", uq.Len())
	}

	got := []string{}
	for uq.Len() != 0 {
		got = append(got, uq.Pop())
	}

	if len(got) != 3 {
		t.Errorf("Expected len=3 got len=%d", len(got))
	}

	if strings.Join(got, ",") != "a,b,c" {
		t.Errorf("Expected a,b,c got %s", strings.Join(got, ","))
	}

}
