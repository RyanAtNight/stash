package config

import (
	"path/filepath"

	"github.com/stashapp/stash/pkg/fsutil"
)

// LibraryTrashFolderName is the name of the trash folder created in each library root
// when per-library trash is enabled.
const LibraryTrashFolderName = ".stash-trash"

// Stash configuration details
type StashConfigInput struct {
	Path         string `json:"path"`
	ExcludeVideo bool   `json:"excludeVideo"`
	ExcludeImage bool   `json:"excludeImage"`
}

type StashConfig struct {
	Path         string `json:"path"`
	ExcludeVideo bool   `json:"excludeVideo"`
	ExcludeImage bool   `json:"excludeImage"`
}

type StashConfigs []*StashConfig

func (s StashConfigs) GetStashFromPath(path string) *StashConfig {
	for _, f := range s {
		if fsutil.IsPathInDir(f.Path, filepath.Dir(path)) {
			return f
		}
	}
	return nil
}

func (s StashConfigs) GetStashFromDirPath(dirPath string) *StashConfig {
	for _, f := range s {
		if fsutil.IsPathInDir(f.Path, dirPath) {
			return f
		}
	}
	return nil
}

// GetTrashPath returns the path to the trash folder for this library.
func (s *StashConfig) GetTrashPath() string {
	return filepath.Join(s.Path, LibraryTrashFolderName)
}
