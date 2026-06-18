package dhroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaults(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults() returned error: %v", err)
	}

	expectations := []struct {
		name  string
		path  string
		isDir bool
	}{
		{"data dir", filepath.Join(home, ".dhi", "data"), true},
		{"config.yaml", filepath.Join(home, ".dhi", "config.yaml"), false},
		{"pipelines.yaml", filepath.Join(home, ".dhi", "pipelines.yaml"), false},
		{"scripts dir", filepath.Join(home, ".dhi", "scripts"), true},
	}

	for _, e := range expectations {
		info, err := os.Stat(e.path)
		if err != nil {
			t.Errorf("expected %s (%s) to exist: %v", e.name, e.path, err)
			continue
		}
		if e.isDir && !info.IsDir() {
			t.Errorf("expected %s (%s) to be a directory, but it is a file", e.name, e.path)
		}
		if !e.isDir && info.IsDir() {
			t.Errorf("expected %s (%s) to be a file, but it is a directory", e.name, e.path)
		}
	}
}

func TestEnsureDefaults_Idempotent(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("first EnsureDefaults() call returned error: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("second EnsureDefaults() call returned error: %v", err)
	}
}

func TestEnsureDefaults_ScriptsAlreadyExist(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })

	if err := EnsureDefaults(); err != nil {
		t.Fatal(err)
	}

	scriptsDir := ScriptsDir()
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected scripts to be installed")
	}

	origData, _ := os.ReadFile(filepath.Join(scriptsDir, entries[0].Name()))
	if err := EnsureDefaults(); err != nil {
		t.Fatal(err)
	}
	afterData, _ := os.ReadFile(filepath.Join(scriptsDir, entries[0].Name()))
	if string(origData) != string(afterData) {
		t.Error("EnsureDefaults should not overwrite existing scripts")
	}
}

func TestEnsureDefaults_ScriptsDirReadOnly(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Chmod(filepath.Join(home, ".dhi", "scripts"), 0755)
	})

	scriptsDir := filepath.Join(home, ".dhi", "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scriptsDir, 0555); err != nil {
		t.Fatal(err)
	}

	err := EnsureDefaults()
	if err == nil {
		t.Fatal("expected error when scripts dir is read-only")
	}
}
