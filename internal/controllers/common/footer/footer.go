package footer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	localResourcev1alpha1 "github.com/krateoplatformops/git-provider/apis/localresource/v1alpha1"
	hasher "github.com/krateoplatformops/git-provider/internal/tools/hash"
	"k8s.io/client-go/dynamic"
)

type localResourceCommitFooter struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	SpecHash  string `json:"specHash"`
}

const localResourceCommitFooterPrefix = "Managed by git-provider LocalResource:"

func CalculateLocalResourceSpecHash(ctx context.Context, cr *localResourcev1alpha1.LocalResource, dyn dynamic.Interface) (string, error) {
	hash := hasher.NewFNVObjectHash()
	err := hash.SumHash(
		cr.Spec.FromResource,
		cr.Spec.ToRepo.Url,
		cr.Spec.ToRepo.Path,
		cr.Spec.ToRepo.Branch,
		cr.Spec.ToRepo.CloneFromBranch,
		cr.Spec.PlaceholdersToOverride,
	)
	if err != nil {
		return "", fmt.Errorf("unable to compute LocalResource spec hash: %w", err)
	}

	if cr.Spec.FromResource.FromRef != nil && dyn != nil {
		// include the resource version of the referenced resource in the hash
		gv, err := schema.ParseGroupVersion(cr.Spec.FromResource.FromRef.ApiVersion)
		if err != nil {
			return "", fmt.Errorf("unable to parse apiVersion %s of referenced resource: %w", cr.Spec.FromResource.FromRef.ApiVersion, err)
		}
		var cli dynamic.ResourceInterface
		if cr.Spec.FromResource.FromRef.Namespace == "" {
			cli = dyn.Resource(schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: cr.Spec.FromResource.FromRef.Resource,
			})
		} else {
			cli = dyn.Resource(schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: cr.Spec.FromResource.FromRef.Resource,
			}).Namespace(cr.Spec.FromResource.FromRef.Namespace)
		}

		u, err := cli.Get(ctx, cr.Spec.FromResource.FromRef.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("unable to get referenced resource %s/%s: %w", cr.Spec.FromResource.FromRef.Namespace, cr.Spec.FromResource.FromRef.Name, err)
		}
		err = hash.SumHash(u.GetResourceVersion())
		if err != nil {
			return "", fmt.Errorf("unable to compute LocalResource spec hash: %w", err)
		}
	}

	return hash.GetHash(), nil
}

func LocalResourceCommitFooter(ctx context.Context, cr *localResourcev1alpha1.LocalResource, dyn dynamic.Interface) (string, error) {
	hshString, err := CalculateLocalResourceSpecHash(ctx, cr, dyn)
	if err != nil {
		return "", fmt.Errorf("unable to compute LocalResource spec hash: %w", err)
	}
	// small struct format for easy parsing
	footerStruct := localResourceCommitFooter{
		Namespace: cr.GetNamespace(),
		Name:      cr.GetName(),
		SpecHash:  hshString,
	}
	b, err := json.Marshal(footerStruct)
	if err != nil {
		return "", fmt.Errorf("unable to marshal LocalResource commit footer: %w", err)
	}
	return fmt.Sprintf("%s %s", localResourceCommitFooterPrefix, string(b)), nil
}

func ParseLocalResourceCommitFooter(footer string) (namespace, name, specHash string, err error) {
	// There could be other text before the footer, so we need to extract the footer part
	footerStart := strings.LastIndex(footer, localResourceCommitFooterPrefix)
	if footerStart == -1 {
		return "", "", "", fmt.Errorf("LocalResource commit footer not found")
	}
	footer = footer[footerStart+len(localResourceCommitFooterPrefix):] // +1 to skip the space
	footer = strings.TrimSpace(footer)
	var footerStruct localResourceCommitFooter
	err = json.Unmarshal([]byte(footer), &footerStruct)
	if err != nil {
		return "", "", "", fmt.Errorf("unable to unmarshal LocalResource commit footer: %w", err)
	}
	ns := footerStruct.Namespace
	nm := footerStruct.Name
	sh := footerStruct.SpecHash
	return ns, nm, sh, nil
}
