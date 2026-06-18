package dhroot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", dir)
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	return dir
}

func TestRoot_ContainsDotDhi(t *testing.T) {
	home := withTempHome(t)
	root := Root()
	want := filepath.Join(home, ".dhi")
	if root != want {
		t.Errorf("Root() = %q, want %q", root, want)
	}
}

func TestDataDir(t *testing.T) {
	home := withTempHome(t)
	got := DataDir()
	want := filepath.Join(home, ".dhi", "data")
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestConfigPath(t *testing.T) {
	home := withTempHome(t)
	got := ConfigPath()
	want := filepath.Join(home, ".dhi", "config.yaml")
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestPipelinePath(t *testing.T) {
	home := withTempHome(t)
	got := PipelinePath()
	want := filepath.Join(home, ".dhi", "pipelines.yaml")
	if got != want {
		t.Errorf("PipelinePath() = %q, want %q", got, want)
	}
}

func TestScriptsDir(t *testing.T) {
	home := withTempHome(t)
	got := ScriptsDir()
	want := filepath.Join(home, ".dhi", "scripts")
	if got != want {
		t.Errorf("ScriptsDir() = %q, want %q", got, want)
	}
}

func TestMkdirAll(t *testing.T) {
	withTempHome(t)
	if err := MkdirAll(); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}
	if _, err := os.Stat(DataDir()); os.IsNotExist(err) {
		t.Errorf("expected DataDir() to exist after MkdirAll(), but it does not")
	}
}

func TestPaths(t *testing.T) {
	home := withTempHome(t)

	tests := []struct {
		name string
		got  func() string
		want string
	}{
		{
			name: "Root",
			got:  Root,
			want: filepath.Join(home, ".dhi"),
		},
		{
			name: "DataDir",
			got:  DataDir,
			want: filepath.Join(home, ".dhi", "data"),
		},
		{
			name: "ConfigPath",
			got:  ConfigPath,
			want: filepath.Join(home, ".dhi", "config.yaml"),
		},
		{
			name: "PipelinePath",
			got:  PipelinePath,
			want: filepath.Join(home, ".dhi", "pipelines.yaml"),
		},
		{
			name: "ScriptsDir",
			got:  ScriptsDir,
			want: filepath.Join(home, ".dhi", "scripts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(); got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestRoot_Fallback(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", fmt.Errorf("mock failure") }
	t.Cleanup(func() { userHomeDir = old })

	root := Root()
	want := filepath.Join(".dhi")
	if root != want {
		t.Errorf("Root() when UserHomeDir fails = %q, want %q", root, want)
	}
}

func TestMkdirAll_Idempotent(t *testing.T) {
	withTempHome(t)

	if err := MkdirAll(); err != nil {
		t.Fatalf("first MkdirAll() returned error: %v", err)
	}
	if err := MkdirAll(); err != nil {
		t.Fatalf("second MkdirAll() returned error: %v", err)
	}

	info, err := os.Stat(DataDir())
	if err != nil {
		t.Fatalf("DataDir() should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("DataDir() is not a directory")
	}
}

func TestRoot_WithUserHomeDirRestored(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", fmt.Errorf("mock") }
	t.Cleanup(func() { userHomeDir = old })

	fallbackRoot := Root()
	if fallbackRoot != filepath.Join(".dhi") {
		t.Fatalf("expected fallback, got %q", fallbackRoot)
	}

	userHomeDir = old
	realRoot := Root()
	if realRoot == filepath.Join(".dhi") {
		t.Error("Root() should not use fallback after userHomeDir is restored")
	}
	if !filepath.IsAbs(realRoot) {
		t.Errorf("expected absolute path for Root(), got %q", realRoot)
	}
}
