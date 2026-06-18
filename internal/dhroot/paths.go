package dhroot

import (
	"os"
	"path/filepath"
)

var userHomeDir = os.UserHomeDir

func Root() string {
	home, err := userHomeDir()
	if err != nil {
		return filepath.Join(".dhi")
	}
	return filepath.Join(home, ".dhi")
}

func DataDir() string {
	return filepath.Join(Root(), "data")
}

func ConfigPath() string {
	return filepath.Join(Root(), "config.yaml")
}

func PipelinePath() string {
	return filepath.Join(Root(), "pipelines.yaml")
}

func ScriptsDir() string {
	return filepath.Join(Root(), "scripts")
}

func MkdirAll() error {
	return os.MkdirAll(DataDir(), 0755)
}
