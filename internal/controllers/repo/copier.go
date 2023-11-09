package repo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/krateoplatformops/git-provider/internal/clients/git"

	gi "github.com/sabhiram/go-gitignore"
)

type copier struct {
	fromRepo       *git.Repo
	toRepo         *git.Repo
	originCopyPath string
	targetCopyPath string
	renderFunc     func(in io.Reader, out io.Writer) error
	krateoIgnore   *gi.GitIgnore
	targetIgnore   *gi.GitIgnore
}

func (co *copier) copyFile(src, dst string, doNotRender bool) (err error) {
	fromFS, toFS := co.fromRepo.FS(), co.toRepo.FS()

	in, err := fromFS.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := toFS.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	if doNotRender || co.renderFunc == nil {
		_, err = io.Copy(out, in)
		return err
	}

	return co.renderFunc(in, out)
}

// copyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func (co *copier) copyDir(src, dst string) (err error) {
	if len(src) == 0 {
		src = "/"
	}

	if len(dst) == 0 {
		dst = "/"
	}

	fromFS, toFS := co.fromRepo.FS(), co.toRepo.FS()

	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := fromFS.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	err = toFS.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := fromFS.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = co.copyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			doNotRender := false
			doNotCopy := false
			if co.krateoIgnore != nil {
				if co.krateoIgnore.MatchesPath(srcPath) {
					doNotRender = true
				}
			}
			if co.targetIgnore != nil {
				relSrc, err := filepath.Rel(co.originCopyPath, srcPath)
				if err != nil {
					return err
				}
				if co.targetIgnore.MatchesPath(filepath.Join(co.targetCopyPath, relSrc)) {
					doNotCopy = true
				}
			}

			// do the copy
			if !doNotCopy {
				err = co.copyFile(srcPath, dstPath, doNotRender)
				if err != nil {
					return
				}
			}
		}
	}

	return
}
