package api

import (
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/scene"
)

// newFileDeleter creates a file.Deleter configured based on the current trash settings.
// If useLibraryTrash is enabled, files will be moved to a .stash-trash folder
// within their respective library directories. Otherwise, files will be moved
// to the global trash path (if set) or permanently deleted.
func newFileDeleter() *file.Deleter {
	mgr := manager.GetInstance()
	c := mgr.Config

	if c.GetUseLibraryTrash() {
		stashPaths := c.GetStashPaths()
		globalTrashPath := c.GetDeleteTrashPath()

		resolver := func(filePath string) string {
			// Find which library this file belongs to
			stash := stashPaths.GetStashFromPath(filePath)
			if stash != nil {
				return stash.GetTrashPath()
			}

			// Fall back to global trash path if file is not in any library
			return globalTrashPath
		}

		return file.NewDeleterWithResolver(resolver)
	}

	// Use global trash path (or permanent deletion if not set)
	return file.NewDeleterWithTrash(c.GetDeleteTrashPath())
}

// newSceneFileDeleter creates a scene.FileDeleter configured with the proper trash settings.
func newSceneFileDeleter() *scene.FileDeleter {
	mgr := manager.GetInstance()
	return &scene.FileDeleter{
		Deleter:        newFileDeleter(),
		FileNamingAlgo: mgr.Config.GetVideoFileNamingAlgorithm(),
		Paths:          mgr.Paths,
	}
}

// newImageFileDeleter creates an image.FileDeleter configured with the proper trash settings.
func newImageFileDeleter() *image.FileDeleter {
	mgr := manager.GetInstance()
	return &image.FileDeleter{
		Deleter: newFileDeleter(),
		Paths:   mgr.Paths,
	}
}
