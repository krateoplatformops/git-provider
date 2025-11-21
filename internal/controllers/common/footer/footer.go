package footer

import (
	"encoding/json"
	"fmt"
	"strings"

	localResourcev1alpha1 "github.com/krateoplatformops/git-provider/apis/localresource/v1alpha1"
	hasher "github.com/krateoplatformops/git-provider/internal/tools/hash"
)

type localResourceCommitFooter struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	SpecHash  string `json:"specHash"`
}

const localResourceCommitFooterPrefix = "Managed by git-provider LocalResource:"

func CalculateLocalResourceSpecHash(cr *localResourcev1alpha1.LocalResource) (string, error) {
	hash := hasher.NewFNVObjectHash()
	err := hash.SumHash(
		cr.Spec.FromResource,
		cr.Spec.ToRepo.Url,
		cr.Spec.ToRepo.Path,
		cr.Spec.ToRepo.Branch,
		cr.Spec.ToRepo.CloneFromBranch,
		cr.Spec.PlaceholdersToOverride)
	if err != nil {
		return "", fmt.Errorf("unable to compute LocalResource spec hash: %w", err)
	}
	return hash.GetHash(), nil
}

func LocalResourceCommitFooter(cr *localResourcev1alpha1.LocalResource) (string, error) {
	hshString, err := CalculateLocalResourceSpecHash(cr)
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
