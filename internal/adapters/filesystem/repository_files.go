// Package filesystem reads the files of a project directory from the host.
package filesystem

import (
	"io/fs"
	"os"
)

// RepositoryFiles reads files and directories through the os package.
type RepositoryFiles struct{}

func NewRepositoryFiles() RepositoryFiles {
	return RepositoryFiles{}
}

func (RepositoryFiles) ReadDir(dir string) ([]fs.DirEntry, error) {
	return os.ReadDir(dir)
}

func (RepositoryFiles) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (RepositoryFiles) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
