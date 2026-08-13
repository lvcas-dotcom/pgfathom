package cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestOnlyTheCommandLayerImportsTheInterface holds the boundary that justified
// the dependency in the first place. The guide is worth 19 extra modules in the
// binary; a catalog reader or an inference layer that imported a terminal
// library would be worth none, and would make the tool untestable without one.
func TestOnlyTheCommandLayerImportsTheInterface(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f",
		"{{if .Module}}{{.ImportPath}} {{.Module.Path}}{{end}}",
		"../../internal/catalog", "../../internal/infer", "../../internal/validate",
		"../../internal/report", "../../internal/discovery", "../../internal/model",
		"../../internal/profile", "../../internal/stats", "../../internal/sqlprobe",
	).Output()
	if err != nil {
		t.Fatalf("listing dependencies: %v", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "charmbracelet") {
			t.Errorf("a layer below the command imports the terminal interface: %s", line)
		}
	}
}
