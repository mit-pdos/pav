package main

import (
	"flag"
	"os"
	"os/exec"
	"path"
	"testing"
)

const (
	dataDir = "testdata"
)

var update = flag.Bool("update", false, "update golden files")

type entry struct {
	source, golden string
}

var data = []entry{
	{"test.input", "test.golden"},
}

func check(t *testing.T, source, golden string) {
	src, err := os.ReadFile(source)
	if err != nil {
		t.Error(err)
		return
	}
	res := compile(src)

	actual, err := os.CreateTemp("", "")
	defer actual.Close()
	if err != nil {
		t.Error(err)
		return
	}
	_, err = actual.Write(res)
	if err != nil {
		t.Error(err)
		return
	}

	cmd := exec.Command("diff", "--unified", actual.Name(), golden)
	diff, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("actual output diff from golden:\n%s", diff)
	}

	if *update {
		if err := os.WriteFile(golden, res, 0644); err != nil {
			t.Error(err)
		}
		t.Log("updated: ", golden)
	}
}

func TestFiles(t *testing.T) {
	t.Parallel()
	for _, e := range data {
		source := path.Join(dataDir, e.source)
		golden := path.Join(dataDir, e.golden)
		t.Run(e.source, func(t *testing.T) {
			t.Parallel()
			check(t, source, golden)
		})
	}
}
