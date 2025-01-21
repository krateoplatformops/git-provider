package repo

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	repov1alpha1 "github.com/krateoplatformops/git-provider/apis/repo/v1alpha1"

	"github.com/cbroglie/mustache"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/krateoplatformops/git-provider/internal/ptr"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"
	gi "github.com/sabhiram/go-gitignore"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type externalClientOpts struct {
	Insecure                bool
	UnsupportedCapabilities bool
	FromRepoCreds           transport.AuthMethod
	ToRepoCreds             transport.AuthMethod
	FromRepoCookieFile      []byte
	ToRepoCookieFile        []byte
}

func loadExternalClientOpts(ctx context.Context, kc client.Client, cr *repov1alpha1.Repo) (*externalClientOpts, error) {
	var fromRepoCookie, toRepoCookie []byte
	fromRepoCreds, err := getRepoCredentials(ctx, kc, cr.Spec.FromRepo)
	if err != nil {
		return nil, fmt.Errorf("retrieving .fromRepo credentials: %w", err)
	}
	fromRepoCookie = nil
	if fromRepoCreds == nil {
		fromRepoCookie, err = getRepoCookies(ctx, kc, cr.Spec.FromRepo)
		if err != nil {
			return nil, fmt.Errorf("retrieving .fromRepo cookies: %w", err)
		}
	}

	toRepoCreds, err := getRepoCredentials(ctx, kc, cr.Spec.ToRepo)
	if err != nil {
		return nil, fmt.Errorf("retrieving .toRepo credentials: %w", err)
	}
	if toRepoCreds == nil {
		toRepoCookie, err = getRepoCookies(ctx, kc, cr.Spec.ToRepo)
		if err != nil {
			return nil, fmt.Errorf("retrieving .toRepo cookies: %w", err)
		}
	}

	// fmt.Println("ToRepoCookieFile", string(toRepoCookie))

	return &externalClientOpts{
		Insecure:                ptr.BoolFromPtr(cr.Spec.Insecure),
		UnsupportedCapabilities: ptr.BoolFromPtr(cr.Spec.UnsupportedCapabilities),
		FromRepoCreds:           fromRepoCreds,
		ToRepoCreds:             toRepoCreds,
		FromRepoCookieFile:      fromRepoCookie,
		ToRepoCookieFile:        toRepoCookie,
	}, nil
}

func getRepoCookies(ctx context.Context, k client.Client, opts repov1alpha1.RepoOpts) ([]byte, error) {
	if opts.SecretRef == nil {
		return nil, nil
	}

	sec, err := resource.GetSecret(ctx, k, opts.SecretRef)

	return []byte(sec), err
}

// getRepoCredentials returns the from repo credentials stored in a secret.
func getRepoCredentials(ctx context.Context, k client.Client, opts repov1alpha1.RepoOpts) (transport.AuthMethod, error) {
	if opts.SecretRef == nil {
		return nil, nil
	}

	token, err := resource.GetSecret(ctx, k, opts.SecretRef)
	if err != nil {
		return nil, err
	}

	authMethod := ptr.StringFromPtr(opts.AuthMethod)
	if strings.EqualFold(authMethod, "bearer") {
		return &githttp.TokenAuth{
			Token: token,
		}, nil
	}

	if strings.EqualFold(authMethod, "cookiefile") {
		return nil, nil
	}

	username := "krateoctl"
	if opts.UsernameRef != nil {
		username, err = resource.GetSecret(ctx, k, opts.UsernameRef)
		if err != nil {
			return nil, err
		}
	}

	return &githttp.BasicAuth{
		Username: username,
		Password: token,
	}, nil
}

func createRenderFuncs(co *copier, values interface{}) {
	co.renderFunc = func(in io.Reader, out io.Writer) error {
		bin, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		tmpl, err := mustache.ParseString(string(bin))
		if err != nil {
			return err
		}

		return tmpl.FRender(out, values)
	}
	co.renderFileNames = func(src string) (string, error) {
		tmpl, err := mustache.ParseString(src)
		if err != nil {
			return "", err
		}
		return tmpl.Render(values)
	}

}

func loadIgnoreFileEventually(co *copier) error {
	fp, err := co.fromRepo.FS().Open(".krateoignore")
	if err != nil {
		return err
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

func loadFilesIntoArray(fs billy.Filesystem, dir string, flist *[]string) error {
	files, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			err := loadFilesIntoArray(fs, filepath.Join(dir, file.Name()), flist)
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

func loadIgnoreTargetFiles(srcPath string, co *copier) error {
	fs := co.toRepo.FS()
	var flist []string
	err := loadFilesIntoArray(fs, srcPath, &flist)
	if err != nil {
		return err
	}
	co.targetIgnore = gi.CompileIgnoreLines(flist...)
	return nil
}
