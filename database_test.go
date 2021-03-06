package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testDir = "tests"

var testFileTime = time.Now().Add(-time.Hour)

// Create a file in the test directory, ensuring that its last modified time
// is one second after the last created file. This is necessary because Make
// only compares file times to the second.
func createTestFile(name string) {
	path := filepath.Join(testDir, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.Close()
	if err := os.Chtimes(path, testFileTime, testFileTime); err != nil {
		panic(err)
	}
	testFileTime = testFileTime.Add(time.Second)
}

func clearTestFiles() {
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		panic(err)
	}
	for _, info := range files {
		if !strings.HasPrefix(info.Name(), "Makefile") {
			os.Remove(filepath.Join(testDir, info.Name()))
		}
	}
}

func runMake() []byte {
	cmd := exec.Command(
		"make",
		"--question",
		"--print-data-base",
	)
	cmd.Dir = testDir
	out, _ := cmd.Output()
	return out
}

func getDatabase() Database {
	out := runMake()
	r := bytes.NewReader(out)
	db := NewDatabase()
	if err := db.Populate(r); err != nil {
		panic(err)
	}
	return db
}

func targetIsMissing(db Database, t *Target) bool {
	// Target does not exist, needs update.
	ok := db.Query(t.Name, time.Now())
	return !ok && t.DoesNotExist && t.NeedsUpdate
}

func targetNeedsUpdate(db Database, t *Target) bool {
	// Target exists, needs update due to dependency.
	ok := db.Query(t.Name, time.Now())
	return !ok && !t.DoesNotExist && t.NeedsUpdate
}

func targetNotChecked(db Database, t *Target) bool {
	// Target was not checked because another dependency needs updating.
	// Target exists, is up to date.
	ok := db.Query(t.Name, time.Now())
	return ok && !t.DoesNotExist && !t.NeedsUpdate
}

func targetOK(db Database, t *Target) bool {
	// Target exists, needs update due to dependency.
	ok := db.Query(t.Name, time.Now())
	return ok && !t.DoesNotExist && !t.NeedsUpdate
}

type TargetAssertions map[string](func(db Database, t *Target) bool)

func (a TargetAssertions) Check() error {
	db := getDatabase()
	for name, checkFunc := range a {
		t := db.Targets[name]
		if !checkFunc(db, t) {
			ok := db.Query(name, time.Now())
			return fmt.Errorf(
				"\nTarget: %s\nOK: %v\nDoesNotExist: %v\nNeedsUpdate: %v",
				t.Name, ok, t.DoesNotExist, t.NeedsUpdate,
			)
		}
	}
	return nil
}

func TestMakeFileTargets(t *testing.T) {
	clearTestFiles()
	defer clearTestFiles()

	// Every target is missing, except for f4, which doesn't get checked.
	// That is because f1 requires f2 which requires f3 and f4.
	// When f3 is found to be missing, there is no need to check f4.

	tests := TargetAssertions{
		"f1": targetIsMissing,
		"f2": targetIsMissing,
		"f3": targetIsMissing,
		"f4": targetNotChecked, // No need to check f4 when f3 is missing.
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

	createTestFile("f1")

	// Now that f1 exists, it should see that it needs updating.
	// That is because it requires f2, which is still missing.

	tests = TargetAssertions{
		"f1": targetNeedsUpdate,
		"f2": targetIsMissing,
		"f3": targetIsMissing,
		"f4": targetNotChecked,
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

	createTestFile("f2")

	// Now that f2 exists, it should now need updating, because it requires
	// the missing f3 and f4. Because f1 requires f2, and f2 needs updating,
	// it still needs updating.

	tests = TargetAssertions{
		"f1": targetNeedsUpdate,
		"f2": targetNeedsUpdate,
		"f3": targetIsMissing,
		"f4": targetNotChecked,
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

	createTestFile("f3")

	// Now that f3 exists, it should be OK because it has no dependencies.
	// It now checks f4 and finds it to be missing, so f1 -> f2 -> f4 is
	// still not OK.

	tests = TargetAssertions{
		"f1": targetNeedsUpdate,
		"f2": targetNeedsUpdate,
		"f3": targetOK,
		"f4": targetIsMissing, // Now f4 is being checked.
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

	createTestFile("f4")

	// Now that f4 exists, it should be OK because it has no dependencies.
	// Because f2 depends on f3 and f4, and they were created AFTER f2,
	// it still needs to be updated.

	tests = TargetAssertions{
		"f1": targetNeedsUpdate,
		"f2": targetNeedsUpdate,
		"f3": targetOK,
		"f4": targetOK,
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

	createTestFile("f2")

	// Now that f2 has been updated after f3 and f4, it is OK. Because f1
	// depends on f2, and f1 was created first, it still needs to be updated.

	tests = TargetAssertions{
		"f1": targetNeedsUpdate,
		"f2": targetOK,
		"f3": targetOK,
		"f4": targetOK,
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

	createTestFile("f1")

	// Now everything should be OK.

	tests = TargetAssertions{
		"f1": targetOK,
		"f2": targetOK,
		"f3": targetOK,
		"f4": targetOK,
	}
	if err := tests.Check(); err != nil {
		t.Error(err)
	}

}
