package dhroot

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed defaultdata/config.yaml defaultdata/pipelines.yaml defaultdata/scripts/*
var defaults embed.FS

func EnsureDefaults() error {
	if err := MkdirAll(); err != nil {
		return err
	}

	if err := ensureFile("config.yaml", ConfigPath(), 0644); err != nil {
		return err
	}

	if err := ensureFile("pipelines.yaml", PipelinePath(), 0644); err != nil {
		return err
	}

	if err := ensureScripts(); err != nil {
		return err
	}

	return nil
}

func ensureFile(srcName, dstPath string, mode os.FileMode) error {
	if _, err := os.Stat(dstPath); err == nil {
		return nil
	}
	data, err := defaults.ReadFile("defaultdata/" + srcName)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, mode)
}

func ensureScripts() error {
	entries, err := defaults.ReadDir("defaultdata/scripts")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ScriptsDir(), 0755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dstPath := filepath.Join(ScriptsDir(), e.Name())
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}
		data, err := fs.ReadFile(defaults, "defaultdata/scripts/"+e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0755); err != nil {
			return err
		}
	}
	return nil
}
