package credentialhelper

import (
	"fmt"
	"strings"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

type CredentialHelperOpts struct {
	AuthMethod string
	Username   string
	Token      string
}

type Credentials struct {
	Transport transport.AuthMethod
	Cookie    []byte
}

func GetCredentials(opts CredentialHelperOpts) (*Credentials, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("token is required for authentication")
	}

	if strings.EqualFold(opts.AuthMethod, "bearer") {
		return &Credentials{
			Transport: &githttp.TokenAuth{
				Token: opts.Token,
			},
			Cookie: nil,
		}, nil
	}

	if strings.EqualFold(opts.AuthMethod, "cookiefile") {
		return &Credentials{
			Transport: nil,
			Cookie:    []byte(opts.Token),
		}, nil
	}

	return &Credentials{
		Transport: &githttp.BasicAuth{
			Username: opts.Username,
			Password: opts.Token,
		},
		Cookie: nil,
	}, nil
}
