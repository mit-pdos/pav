package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"testing"
)

const (
	dataDir = "testdata"
)

var update = flag.Bool("update", false, "update golden files")

var dump = flag.Bool("dump", false, "dump golden ast [for dev]")

type entry struct {
	source, golden string
	runs           int
}

var data = []entry{
	{"types/types.go", "types/types.golden", 1},
	{"alias/alias.go", "alias/alias.golden", 1},
	{"mult/mult.go", "mult/mult.golden", 3},
	{"nogen/nogen.go", "nogen/nogen.golden", 1},
	{"const/const.go", "const/const.golden", 1},
}

// tmpWrite writes data to a tmp file and returns the tmp file name.
func tmpWrite(t *testing.T, data []byte) string {
	f, err := os.CreateTemp("", "")
	defer func() {
		err := f.Close()
		if err != nil {
			t.Error(err)
		}
	}()
	if err != nil {
		t.Error(err)
		return ""
	}
	_, err = f.Write(data)
	if err != nil {
		t.Error(err)
		return ""
	}
	return f.Name()
}

func check(t *testing.T, source, golden string) {
	if *dump {
		gld, err := os.ReadFile(golden)
		if err != nil {
			t.Error(err)
			return
		}
		res := printAst(t, gld)
		t.Logf("golden ast dump:\n%s", res)
	}

	res := compile(source)
	actual := tmpWrite(t, res)

	cmd := exec.Command("diff", "--unified", actual, golden)
	diff, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("actual output diff from golden:\n%s", diff)
	}

	err = os.Remove(actual)
	if err != nil {
		t.Error(err)
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
			for i := 0; i < e.runs && !t.Failed(); i++ {
				check(t, source, golden)
			}
		})
	}
}

func printAst(t *testing.T, src []byte) []byte {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Error(err)
	}
	res := new(bytes.Buffer)
	ast.Fprint(res, fset, f, ast.NotNilFilter)
	return res.Bytes()
}
