package colors

import "testing"

func TestColors(t *testing.T) {
	s := Red("RED")
	if s != "\033[0;31mRED\033[0m" {
		t.Errorf("Got: %s", s)
	}
	s = Yellow("YELLOW")
	if s != "\033[0;33mYELLOW\033[0m" {
		t.Errorf("Got: %s", s)
	}
}
