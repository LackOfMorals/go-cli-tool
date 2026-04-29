package skill

import (
	"io/fs"
	"os"
)

// Filesystem is the narrow set of filesystem primitives the skill service
// needs. It exists so tests can inject a fake or t.TempDir-backed wrapper
// without touching the real $HOME. The default implementation, OSFilesystem,
// delegates to the os package.
type Filesystem interface {
	// Stat follows symlinks, like os.Stat.
	Stat(name string) (fs.FileInfo, error)
	// Lstat does not follow symlinks, like os.Lstat. Used to detect symlinked
	// install targets that must be removed before writing.
	Lstat(name string) (fs.FileInfo, error)
	// MkdirAll creates name and any missing parents with perm, like os.MkdirAll.
	MkdirAll(name string, perm fs.FileMode) error
	// WriteFile writes data to name with perm, truncating if it exists,
	// like os.WriteFile.
	WriteFile(name string, data []byte, perm fs.FileMode) error
	// RemoveAll removes name and any children, like os.RemoveAll. Missing
	// targets are not an error.
	RemoveAll(name string) error
	// Remove removes a single file or empty directory, like os.Remove.
	Remove(name string) error
}

// OSFilesystem is the production Filesystem. Methods forward verbatim to the
// os package.
type OSFilesystem struct{}

// Stat implements Filesystem.
func (OSFilesystem) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

// Lstat implements Filesystem.
func (OSFilesystem) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }

// MkdirAll implements Filesystem.
func (OSFilesystem) MkdirAll(name string, perm fs.FileMode) error {
	return os.MkdirAll(name, perm)
}

// WriteFile implements Filesystem.
func (OSFilesystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// RemoveAll implements Filesystem.
func (OSFilesystem) RemoveAll(name string) error { return os.RemoveAll(name) }

// Remove implements Filesystem.
func (OSFilesystem) Remove(name string) error { return os.Remove(name) }
