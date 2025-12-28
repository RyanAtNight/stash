package fsutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TrashMetadataSuffix is the file extension for sidecar metadata files
const TrashMetadataSuffix = ".stash-meta.json"

// TrashMetadata contains information about a trashed file for restoration
type TrashMetadata struct {
	OriginalPath string    `json:"originalPath"`
	DeletedAt    time.Time `json:"deletedAt"`
}

// WriteTrashMetadata creates a sidecar JSON file with metadata for a trashed file
func WriteTrashMetadata(trashPath, originalPath string) error {
	metadata := TrashMetadata{
		OriginalPath: originalPath,
		DeletedAt:    time.Now(),
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trash metadata: %w", err)
	}

	metaPath := trashPath + TrashMetadataSuffix
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write trash metadata: %w", err)
	}

	return nil
}

// ReadTrashMetadata reads the sidecar metadata file for a trashed file
func ReadTrashMetadata(trashPath string) (*TrashMetadata, error) {
	metaPath := trashPath + TrashMetadataSuffix

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trash metadata: %w", err)
	}

	var metadata TrashMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse trash metadata: %w", err)
	}

	return &metadata, nil
}

// RestoreFromTrash moves a file from trash back to its original location
func RestoreFromTrash(trashPath string) (string, error) {
	// Read metadata to get original path
	metadata, err := ReadTrashMetadata(trashPath)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata for restore: %w", err)
	}

	originalPath := metadata.OriginalPath

	// Check if original path is already occupied
	if _, err := os.Stat(originalPath); err == nil {
		return "", fmt.Errorf("cannot restore: file already exists at %s", originalPath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(originalPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Move file back to original location
	if err := SafeMove(trashPath, originalPath); err != nil {
		return "", fmt.Errorf("failed to restore file: %w", err)
	}

	// Delete the sidecar metadata file
	metaPath := trashPath + TrashMetadataSuffix
	if err := os.Remove(metaPath); err != nil {
		// Log warning but don't fail - the file was restored successfully
		// The orphaned metadata file can be cleaned up later
	}

	return originalPath, nil
}

// MoveToTrash moves a file or directory to a custom trash directory.
// If a file with the same name already exists in the trash, a timestamp is appended.
// Creates a sidecar metadata file to enable restoration.
// Returns the destination path where the file was moved to.
func MoveToTrash(sourcePath string, trashPath string) (string, error) {
	// Get absolute path for the source
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure trash directory exists
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Get the base name of the file/directory
	baseName := filepath.Base(absSourcePath)
	destPath := filepath.Join(trashPath, baseName)

	// If a file with the same name already exists in trash, append timestamp
	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(baseName)
		nameWithoutExt := baseName[:len(baseName)-len(ext)]
		timestamp := time.Now().Format("20060102-150405")
		destPath = filepath.Join(trashPath, fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext))
	}

	// Move the file to trash using SafeMove to support cross-filesystem moves
	if err := SafeMove(absSourcePath, destPath); err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}

	// Create sidecar metadata file for restoration
	if err := WriteTrashMetadata(destPath, absSourcePath); err != nil {
		// Log warning but don't fail - the file was trashed successfully
		// Restoration just won't be possible without metadata
	}

	return destPath, nil
}

// SanitizePathToSlug converts a filesystem path to a safe folder name.
// Colons are removed, path separators become underscores.
// Example: "D:\Media\Videos" -> "D_Media_Videos"
func SanitizePathToSlug(path string) string {
	// Normalize path separators
	slug := filepath.ToSlash(path)

	// Remove drive letter colon (Windows)
	slug = strings.ReplaceAll(slug, ":", "")

	// Replace slashes with underscores
	slug = strings.ReplaceAll(slug, "/", "_")

	// Remove leading/trailing underscores
	slug = strings.Trim(slug, "_")

	// Replace multiple underscores with single underscore
	re := regexp.MustCompile(`_+`)
	slug = re.ReplaceAllString(slug, "_")

	return slug
}

// CreateTrashSymlinks creates symlinks in globalTrashPath pointing to each library's trash folder.
// On Windows, this creates junction points. On Unix, this creates symbolic links.
// The symlink names are derived from the library path using SanitizePathToSlug.
func CreateTrashSymlinks(globalTrashPath string, libraryPaths []string, trashFolderName string) error {
	if globalTrashPath == "" {
		return nil
	}

	// Ensure global trash directory exists
	if err := os.MkdirAll(globalTrashPath, 0755); err != nil {
		return fmt.Errorf("failed to create global trash directory: %w", err)
	}

	for _, libPath := range libraryPaths {
		libTrashPath := filepath.Join(libPath, trashFolderName)
		symlinkName := SanitizePathToSlug(libPath)
		symlinkPath := filepath.Join(globalTrashPath, symlinkName)

		// Check if symlink already exists and points to the correct location
		if target, err := os.Readlink(symlinkPath); err == nil {
			if filepath.Clean(target) == filepath.Clean(libTrashPath) {
				continue // Symlink already correct
			}
			// Remove incorrect symlink
			if err := os.Remove(symlinkPath); err != nil {
				return fmt.Errorf("failed to remove stale symlink %s: %w", symlinkPath, err)
			}
		}

		// Remove any existing file/directory at the symlink path
		if _, err := os.Lstat(symlinkPath); err == nil {
			if err := os.Remove(symlinkPath); err != nil {
				return fmt.Errorf("failed to remove existing path %s: %w", symlinkPath, err)
			}
		}

		// Ensure library trash directory exists before creating symlink
		if err := os.MkdirAll(libTrashPath, 0755); err != nil {
			return fmt.Errorf("failed to create library trash directory %s: %w", libTrashPath, err)
		}

		// Create symlink (on Windows, os.Symlink creates a junction for directories)
		if err := os.Symlink(libTrashPath, symlinkPath); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", symlinkPath, libTrashPath, err)
		}
	}

	return nil
}

// CleanupStaleTrashSymlinks removes symlinks in globalTrashPath that point to
// libraries no longer in the configuration.
func CleanupStaleTrashSymlinks(globalTrashPath string, libraryPaths []string) error {
	if globalTrashPath == "" {
		return nil
	}

	// Build set of valid symlink names
	validNames := make(map[string]bool)
	for _, libPath := range libraryPaths {
		validNames[SanitizePathToSlug(libPath)] = true
	}

	entries, err := os.ReadDir(globalTrashPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read global trash directory: %w", err)
	}

	for _, entry := range entries {
		// Check if this is a symlink
		symlinkPath := filepath.Join(globalTrashPath, entry.Name())
		info, err := os.Lstat(symlinkPath)
		if err != nil {
			continue
		}

		// Only process symlinks
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		// If symlink name is not in valid set, remove it
		if !validNames[entry.Name()] {
			if err := os.Remove(symlinkPath); err != nil {
				return fmt.Errorf("failed to remove stale symlink %s: %w", symlinkPath, err)
			}
		}
	}

	return nil
}
