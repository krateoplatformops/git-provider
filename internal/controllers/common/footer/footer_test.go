package footer

import (
	"fmt"
	"testing"

	localResourcev1alpha1 "github.com/krateoplatformops/git-provider/apis/localresource/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeTestLocalResource() *localResourcev1alpha1.LocalResource {
	return &localResourcev1alpha1.LocalResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-name",
		},
		Spec: localResourcev1alpha1.LocalResourceSpec{
			FromResource: localResourcev1alpha1.FromResource{
				FileName: "config.yaml",
			},
			ToRepo: localResourcev1alpha1.LocalResourceOpts{
				Url:             "https://example.com/repo.git",
				Path:            "path/in/repo",
				Branch:          "main",
				CloneFromBranch: "develop",
			},
			PlaceholdersToOverride: []localResourcev1alpha1.Placeholder{
				{
					Name:  "PLACEHOLDER_1",
					Value: "value1",
				},
				{
					Name:  "PLACEHOLDER_2",
					Value: "value2",
				},
			},
		},
	}
}

func TestLocalResourceCommitFooterAndParse(t *testing.T) {
	cr := makeTestLocalResource()

	footerStr, err := LocalResourceCommitFooter(cr)
	if err != nil {
		t.Fatalf("LocalResourceCommitFooter returned error: %v", err)
	}

	fmt.Println("Generated footer:", footerStr)

	// ensure parse works even if there is other text before the footer
	withPrefix := "Some message\n" + footerStr
	ns, name, specHash, err := ParseLocalResourceCommitFooter(withPrefix)
	if err != nil {
		t.Fatalf("parseLocalResourceCommitFooter returned error: %v", err)
	}

	if ns != cr.GetNamespace() {
		t.Fatalf("namespace mismatch: got %q want %q", ns, cr.GetNamespace())
	}
	if name != cr.GetName() {
		t.Fatalf("name mismatch: got %q want %q", name, cr.GetName())
	}

	expectedHash, err := CalculateLocalResourceSpecHash(cr)
	if err != nil {
		t.Fatalf("CalculateLocalResourceSpecHash returned error: %v", err)
	}
	if specHash != expectedHash {
		t.Fatalf("spec hash mismatch: got %q want %q", specHash, expectedHash)
	}
}

func TestParseLocalResourceCommitFooter_NotFound(t *testing.T) {
	_, _, _, err := ParseLocalResourceCommitFooter("no footer present here")
	if err == nil {
		t.Fatal("expected error when parsing missing footer, got nil")
	}
}

func TestCalculateLocalResourceSpecHash_Deterministic(t *testing.T) {
	cr1 := makeTestLocalResource()
	cr2 := makeTestLocalResource()

	h1, err := CalculateLocalResourceSpecHash(cr1)
	if err != nil {
		t.Fatalf("calculateLocalResourceSpecHash returned error: %v", err)
	}
	h2, err := CalculateLocalResourceSpecHash(cr2)
	if err != nil {
		t.Fatalf("calculateLocalResourceSpecHash returned error: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hash not deterministic for identical specs: %q vs %q", h1, h2)
	}

	// change a field and expect a different hash
	cr2.Spec.ToRepo.Path = "different-path"
	h3, err := CalculateLocalResourceSpecHash(cr2)
	if err != nil {
		t.Fatalf("calculateLocalResourceSpecHash returned error: %v", err)
	}
	if h3 == h1 {
		t.Fatalf("hash did not change after modifying spec: %q == %q", h1, h3)
	}
}
