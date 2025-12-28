package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
)

func (r *mutationResolver) RestoreFromTrash(ctx context.Context, id string) (*RestoreResult, error) {
	// Decode ID to get trash path
	trashPathBytes, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		return &RestoreResult{
			Success: false,
			Error:   ptrStr("Invalid trash file ID"),
		}, nil
	}
	trashPath := string(trashPathBytes)

	// Check if file exists
	if _, err := os.Stat(trashPath); os.IsNotExist(err) {
		return &RestoreResult{
			Success: false,
			Error:   ptrStr("File not found in trash"),
		}, nil
	}

	// Restore the file
	restoredPath, err := fsutil.RestoreFromTrash(trashPath)
	if err != nil {
		return &RestoreResult{
			Success: false,
			Error:   ptrStr(err.Error()),
		}, nil
	}

	logger.Infof("Restored file from trash to %s", restoredPath)

	return &RestoreResult{
		Success:      true,
		RestoredPath: &restoredPath,
	}, nil
}

func (r *mutationResolver) RestoreMultipleFromTrash(ctx context.Context, ids []string) (*RestoreMultipleResult, error) {
	var restored, failed int
	var errors []string

	for _, id := range ids {
		result, err := r.RestoreFromTrash(ctx, id)
		if err != nil {
			failed++
			errors = append(errors, err.Error())
			continue
		}

		if result.Success {
			restored++
		} else {
			failed++
			if result.Error != nil {
				errors = append(errors, *result.Error)
			}
		}
	}

	return &RestoreMultipleResult{
		Restored: restored,
		Failed:   failed,
		Errors:   errors,
	}, nil
}

func (r *mutationResolver) PermanentlyDeleteFromTrash(ctx context.Context, ids []string) (bool, error) {
	for _, id := range ids {
		// Decode ID to get trash path
		trashPathBytes, err := base64.URLEncoding.DecodeString(id)
		if err != nil {
			logger.Warnf("Invalid trash file ID: %s", id)
			continue
		}
		trashPath := string(trashPathBytes)

		// Delete the file
		if err := os.RemoveAll(trashPath); err != nil {
			logger.Warnf("Failed to delete %s: %v", trashPath, err)
		}

		// Delete the metadata sidecar if it exists
		metaPath := trashPath + fsutil.TrashMetadataSuffix
		if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("Failed to delete metadata %s: %v", metaPath, err)
		}
	}

	return true, nil
}

func (r *mutationResolver) EmptyTrash(ctx context.Context, libraryPath *string) (bool, error) {
	mgr := manager.GetInstance()
	c := mgr.Config

	if libraryPath != nil {
		// Empty specific library trash
		stashPaths := c.GetStashPaths()
		for _, stash := range stashPaths {
			if stash.Path == *libraryPath {
				trashPath := stash.GetTrashPath()
				if err := emptyTrashFolder(trashPath); err != nil {
					return false, err
				}
				return true, nil
			}
		}
		return false, fmt.Errorf("library not found: %s", *libraryPath)
	}

	// Empty all library trash folders
	stashPaths := c.GetStashPaths()
	for _, stash := range stashPaths {
		trashPath := stash.GetTrashPath()
		if err := emptyTrashFolder(trashPath); err != nil {
			logger.Warnf("Failed to empty trash for %s: %v", stash.Path, err)
		}
	}

	// Also empty global trash if configured (only non-symlink items)
	globalTrashPath := c.GetDeleteTrashPath()
	if globalTrashPath != "" {
		entries, err := os.ReadDir(globalTrashPath)
		if err == nil {
			for _, entry := range entries {
				filePath := filepath.Join(globalTrashPath, entry.Name())

				// Skip symlinks (these point to library trash folders)
				info, err := os.Lstat(filePath)
				if err != nil {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					continue
				}

				// Skip metadata files - they'll be deleted with their parent
				if strings.HasSuffix(entry.Name(), fsutil.TrashMetadataSuffix) {
					continue
				}

				// Delete the file
				if err := os.RemoveAll(filePath); err != nil {
					logger.Warnf("Failed to delete %s: %v", filePath, err)
				}

				// Delete metadata sidecar
				metaPath := filePath + fsutil.TrashMetadataSuffix
				os.Remove(metaPath)
			}
		}
	}

	return true, nil
}

func emptyTrashFolder(trashPath string) error {
	entries, err := os.ReadDir(trashPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		filePath := filepath.Join(trashPath, entry.Name())
		if err := os.RemoveAll(filePath); err != nil {
			logger.Warnf("Failed to delete %s: %v", filePath, err)
		}
	}

	return nil
}

func ptrStr(s string) *string {
	return &s
}
