package copier

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cbroglie/mustache"
	"github.com/go-git/go-billy/v5"
	"github.com/krateoplatformops/git-provider/internal/tools/template"

	gi "github.com/sabhiram/go-gitignore"
)

const IgnoreFileName = ".krateoignore"

type Option func(*options)
type options struct {
	renderFunc      func(in io.Reader, out io.Writer) error
	renderFileNames func(src string) (string, error)
	ignorePath      string
	originCopyPath  string
	targetCopyPath  string
}

func defaultOptions() options {
	return options{
		renderFunc:      nil,
		renderFileNames: nil,
		ignorePath:      "/",
		originCopyPath:  "/",
		targetCopyPath:  "/",
	}
}

func WithIgnorePath(ip string) Option {
	return func(o *options) {
		o.ignorePath = ip
	}
}

func WithOriginCopyPath(ocp string) Option {
	return func(o *options) {
		o.originCopyPath = ocp
	}
}

func WithTargetCopyPath(tcp string) Option {
	return func(o *options) {
		o.targetCopyPath = tcp
	}
}

func WithGoTemplate(templateValues []template.TemplateValue) Option {
	return func(o *options) {
		values := template.FromTemplateValues(templateValues)
		o.renderFunc = func(in io.Reader, out io.Writer) error {
			bin, err := io.ReadAll(in)
			if err != nil {
				return err
			}
			tmpl := template.Template(bin)
			renderedBin, err := tmpl.Render(values)
			if err != nil {
				return err
			}

			_, err = out.Write(renderedBin)
			return err
		}
		o.renderFileNames = func(src string) (string, error) {
			tmpl := template.Template(src)
			renderedBin, err := tmpl.Render(values)
			if err != nil {
				return "", err
			}

			return string(renderedBin), nil
		}
	}
}

func WithMustacheTemplate(templateValues interface{}) Option {
	return func(o *options) {
		o.renderFunc = func(in io.Reader, out io.Writer) error {
			bin, err := io.ReadAll(in)
			if err != nil {
				return err
			}
			tmpl, err := mustache.ParseString(string(bin))
			if err != nil {
				return err
			}

			return tmpl.FRender(out, templateValues)
		}
		o.renderFileNames = func(src string) (string, error) {
			tmpl, err := mustache.ParseString(src)
			if err != nil {
				return "", err
			}

			return tmpl.Render(templateValues)
		}
	}
}

type Copier struct {
	fromFS       billy.Filesystem
	toFS         billy.Filesystem
	krateoIgnore *gi.GitIgnore
	targetIgnore *gi.GitIgnore

	options
}

func NewCopier(fromFS, toFS billy.Filesystem, opts ...Option) (*Copier, error) {
	if fromFS == nil {
		return nil, fmt.Errorf("fromFS cannot be nil")
	}
	if toFS == nil {
		return nil, fmt.Errorf("toFS cannot be nil")
	}

	options := defaultOptions()
	for _, o := range opts {
		o(&options)
	}

	return &Copier{
		fromFS:  fromFS,
		toFS:    toFS,
		options: options,
	}, nil
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func (co *Copier) Copy(override bool) (err error) {
	src := co.originCopyPath
	dst := co.targetCopyPath

	if len(src) == 0 {
		src = "/"
	}

	if len(dst) == 0 {
		dst = "/"
	}

	if !override {
		err = co.setTargetIgnore()
		if err != nil {
			return fmt.Errorf("failed to set target ignore: %w", err)
		}
	}

	err = co.setKrateoIgnore()
	if err != nil {
		return fmt.Errorf("failed to set krateo ignore: %w", err)
	}

	return co.copyDir(src, dst)
}

func (co *Copier) copyDir(src, dst string) (err error) {
	fromFS, toFS := co.fromFS, co.toFS
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := fromFS.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	doNotRender := false
	doNotCopy := false
	if co.krateoIgnore != nil {
		if co.krateoIgnore.MatchesPath(src) {
			doNotRender = true
		}
	}
	if co.targetIgnore != nil {
		relSrc, err := filepath.Rel(co.originCopyPath, src)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		if co.targetIgnore.MatchesPath(filepath.Join(co.targetCopyPath, relSrc)) {
			doNotCopy = true
		}
	}
	if doNotCopy {
		return
	}
	if !doNotRender && co.renderFileNames != nil {
		dst, err = co.renderFileNames(dst)
		if err != nil {
			return fmt.Errorf("failed to render file names: %w", err)
		}
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
					return fmt.Errorf("failed to get relative path: %w", err)
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
	return nil
}

func (co *Copier) copyFile(src, dst string, doNotRender bool) (err error) {
	fromFS, toFS := co.fromFS, co.toFS

	if !doNotRender && co.renderFileNames != nil {
		var err error
		dst, err = co.renderFileNames(dst)
		if err != nil {
			return fmt.Errorf("failed to render file names: %w", err)
		}
	}

	in, err := fromFS.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer in.Close()

	out, err := toFS.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
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

func (co *Copier) setKrateoIgnore() error {
	fp, err := co.fromFS.Open(filepath.Join(co.ignorePath, IgnoreFileName))
	if err != nil {
		if os.IsNotExist(err) {
			// .krateoignore does not exist, no files to ignore
			return nil
		}
		return fmt.Errorf("unable to open .krateoignore: %w", err)
	}
	defer fp.Close()

	bs, err := io.ReadAll(fp)
	if err != nil {
		return err
	}
	lines := strings.Split(string(bs), "\n")
	co.krateoIgnore = gi.CompileIgnoreLines(lines...)
	return nil
}

func (co *Copier) setTargetIgnore() error {
	if _, err := co.toFS.Stat(co.targetCopyPath); err == nil {
		var flist []string
		err = loadFilesFromPath(co.toFS, co.targetCopyPath, &flist)
		if err != nil {
			return fmt.Errorf("unable to load target files from path: %w", err)
		}
		co.targetIgnore = gi.CompileIgnoreLines(flist...)
	} else if os.IsNotExist(err) {
		// Target path does not exist, no files to ignore
		return nil
	} else {
		return fmt.Errorf("unable to check target path: %w", err)
	}
	return nil
}

func loadFilesFromPath(fs billy.Filesystem, dir string, flist *[]string) error {
	files, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			err := loadFilesFromPath(fs, filepath.Join(dir, file.Name()), flist)
			if err != nil {
				return err
			}
		} else {
			absPath := filepath.Join(dir, file.Name())
			*flist = append(*flist, absPath)
		}
	}

	return nil
}
