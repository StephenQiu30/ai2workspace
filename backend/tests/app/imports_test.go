package app_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestImportBoundaries(t *testing.T) {
	const module = "github.com/StephenQiu30/ai2workspace/backend"
	const internal = module + "/internal/"
	cmd := exec.CommandContext(t.Context(), "go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "./cmd/...", "./internal/...")
	cmd.Dir = "../.."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list application packages: %v\n%s", err, output)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		imports := strings.Fields(line)
		if len(imports) == 0 {
			continue
		}
		from := imports[0]
		root, layer := moduleLayer(strings.TrimPrefix(from, internal))
		command := strings.HasPrefix(from, module+"/cmd/")
		for _, to := range imports[1:] {
			relative, projectImport := strings.CutPrefix(to, internal)
			target, targetLayer := moduleLayer(relative)
			if projectImport {
				switch {
				case target == "app" && command:
					continue
				case target == "infra" && (root == "app" || command):
					continue
				case target != root && (targetLayer != "" || target == "app"):
					t.Errorf("%s imports %s. Cross-module imports must use a business module root.", from, to)
				case layer == "domain" && (root != target || targetLayer != "domain"):
					t.Errorf("%s imports %s. Domain packages can only depend on their module's domain.", from, to)
				case layer == "application" && targetLayer == "adapter":
					t.Errorf("%s imports %s. Application packages depend on ports.", from, to)
				}
				continue
			}
			if (layer == "application" || layer == "domain") &&
				(to == "net/http" || strings.HasPrefix(to, "github.com/gin-gonic/") || strings.HasPrefix(to, "github.com/jackc/pgx/")) {
				t.Errorf("%s imports %s. Transport and storage frameworks belong in adapters.", from, to)
			}
		}
	}
}

func moduleLayer(relative string) (root, layer string) {
	root, rest, _ := strings.Cut(relative, "/")
	layer, _, _ = strings.Cut(rest, "/")
	return root, layer
}
