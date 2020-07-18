package makecmd

import (
	"strings"
	"testing"

	"../makedb"
)

func TestGetFiles(t *testing.T) {
	cmd := Cmd{
		db: &makedb.Database{
			DefaultGoal: "t1",
			Targets: map[string]*makedb.Target{
				"t1": &makedb.Target{
					Name:                "t1",
					NormalPrerequisites: []string{"t2"},
				},
				"t2": &makedb.Target{
					Name: "t2",
					OrderOnlyPrerequisites: []string{"t3"},
				},
				"t3": &makedb.Target{
					Name: "t3",
				},
			},
		},
	}

	cmd.Target = ""
	expected := "t1,t2,t3"
	got := strings.Join(cmd.GetFiles(), ",")
	if got != expected {
		t.Errorf("Expected %s but got %s", expected, got)
	}

	cmd.Target = "t2"
	expected = "t2,t3"
	got = strings.Join(cmd.GetFiles(), ",")
	if got != expected {
		t.Errorf("Expected %s but got %s", expected, got)
	}
}
