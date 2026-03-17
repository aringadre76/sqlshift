//go:build integration

package integration

import (
	"bytes"
	"os/exec"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	// In some environments (notably WSL), XDG_RUNTIME_DIR may be unset or set to
	// a literal placeholder like "$XDG_RUNTIME_DIR", which testcontainers will
	// reject during Docker host detection.
	if xdg == "" || xdg == "$XDG_RUNTIME_DIR" || xdg == "${XDG_RUNTIME_DIR}" {
		if dir, err := os.MkdirTemp("", "sqlshift-xdg-runtime-"); err == nil {
			_ = os.Setenv("XDG_RUNTIME_DIR", dir)
			defer os.RemoveAll(dir)
		}
	} else {
		if st, err := os.Stat(xdg); err != nil || !st.IsDir() {
			if dir, err := os.MkdirTemp("", "sqlshift-xdg-runtime-"); err == nil {
				_ = os.Setenv("XDG_RUNTIME_DIR", dir)
				defer os.RemoveAll(dir)
			}
		} else {
			// Ensure it isn't a symlink to nowhere; keep as-is if it exists.
			_, _ = filepath.EvalSymlinks(xdg)
		}
	}

	// In some WSL/Docker Desktop setups, DOCKER_HOST is configured with an
	// unexpanded placeholder (e.g. unix://$XDG_RUNTIME_DIR/docker.sock). That
	// makes testcontainers panic during docker host detection. If the value still
	// contains '$', rewrite it to the standard Unix socket path.
	if dockerHost := os.Getenv("DOCKER_HOST"); strings.Contains(dockerHost, "$") {
		_ = os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	}

	// If Docker isn't actually reachable in this environment, gracefully skip
	// all integration tests instead of panicking deep inside testcontainers.
	if !dockerReachable() {
		// Print a single line so it's obvious why the tests were skipped.
		_, _ = os.Stderr.WriteString("skipping integration tests: Docker daemon not reachable from WSL\n")
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func dockerReachable() bool {
	path, err := exec.LookPath("docker")
	if err != nil {
		return false
	}

	cmd := exec.Command(path, "info")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

