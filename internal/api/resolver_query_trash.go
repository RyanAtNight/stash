package api

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/fsutil"
)

func (r *queryResolver) TrashContents(ctx context.Context, filter *TrashFilterInput) ([]*TrashedFile, error) {
	mgr := manager.GetInstance()
	c := mgr.Config

	stashPaths := c.GetStashPaths()
	var result []*TrashedFile

	for _, stash := range stashPaths {
		// If filter is set, only include matching library
		if filter != nil && filter.LibraryPath != nil && *filter.LibraryPath != stash.Path {
			continue
		}

		trashPath := stash.GetTrashPath()

		// Read trash directory
		entries, err := os.ReadDir(trashPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Trash folder doesn't exist yet for this library
			}
			return nil, err
		}

		for _, entry := range entries {
			// Skip metadata sidecar files
			if strings.HasSuffix(entry.Name(), fsutil.TrashMetadataSuffix) {
				continue
			}

			filePath := filepath.Join(trashPath, entry.Name())

			// Get file info
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Read metadata sidecar if it exists
			var originalPath string
			var deletedAt time.Time
			metadata, err := fsutil.ReadTrashMetadata(filePath)
			if err == nil {
				originalPath = metadata.OriginalPath
				deletedAt = metadata.DeletedAt
			} else {
				// No metadata - use placeholder values
				originalPath = ""
				deletedAt = info.ModTime()
			}

			// Create ID from base64-encoded trash path
			id := base64.URLEncoding.EncodeToString([]byte(filePath))

			trashedFile := &TrashedFile{
				ID:           id,
				TrashPath:    filePath,
				OriginalPath: originalPath,
				FileName:     entry.Name(),
				LibraryPath:  stash.Path,
				DeletedAt:    deletedAt,
				FileSize:     info.Size(),
			}

			result = append(result, trashedFile)
		}
	}

	// Also check global trash path if configured
	globalTrashPath := c.GetDeleteTrashPath()
	if globalTrashPath != "" && (filter == nil || filter.LibraryPath == nil) {
		entries, err := os.ReadDir(globalTrashPath)
		if err == nil {
			for _, entry := range entries {
				// Skip metadata sidecar files and symlinks to library trash folders
				if strings.HasSuffix(entry.Name(), fsutil.TrashMetadataSuffix) {
					continue
				}

				// Skip symlinks (these are links to library trash folders)
				filePath := filepath.Join(globalTrashPath, entry.Name())
				info, err := os.Lstat(filePath)
				if err != nil {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					continue
				}

				// Get file info
				fileInfo, err := entry.Info()
				if err != nil {
					continue
				}

				// Read metadata sidecar if it exists
				var originalPath string
				var deletedAt time.Time
				metadata, err := fsutil.ReadTrashMetadata(filePath)
				if err == nil {
					originalPath = metadata.OriginalPath
					deletedAt = metadata.DeletedAt
				} else {
					originalPath = ""
					deletedAt = fileInfo.ModTime()
				}

				id := base64.URLEncoding.EncodeToString([]byte(filePath))

				trashedFile := &TrashedFile{
					ID:           id,
					TrashPath:    filePath,
					OriginalPath: originalPath,
					FileName:     entry.Name(),
					LibraryPath:  config.LibraryTrashFolderName, // Indicate global trash
					DeletedAt:    deletedAt,
					FileSize:     fileInfo.Size(),
				}

				result = append(result, trashedFile)
			}
		}
	}

	return result, nil
}
