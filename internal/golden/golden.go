// Package golden compares test output with files in testdata/golden.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update is set by the -update test flag. Because the flag is registered here, it exists only in
// test binaries of packages that import golden: run `go test ./internal/<pkg> -update` per package.
var update = flag.Bool("update", false, "rewrite golden files")

// Assert compares got with testdata/golden/<name>.txt; -update rewrites it.
func Assert(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name+".txt")
	if *update {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
