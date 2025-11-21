package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewLocalFS_Success(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	if info, err := os.Stat(lfs.tmp); err != nil {
		t.Fatalf("tmp dir missing: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("tmp is not a directory")
	}

	f, err := lfs.Create("test.txt")
	if err != nil {
		t.Fatalf("create file error: %v", err)
	}
	_, err = f.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(lfs.tmp, "test.txt"))
	if err != nil {
		t.Fatalf("read file error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestNewLocalFS_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	tmpFile, err := os.CreateTemp(dir, "file-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if _, err := NewLocalFS(tmpPath); err == nil {
		t.Fatalf("expected error when creating temp dir inside a file path, got nil")
	}
}

func TestCleanup_RemovesDir(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tmp := lfs.tmp
	if err := lfs.Cleanup(); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}

	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("expected directory to be removed, but it still exists")
		}
		t.Fatalf("unexpected stat error: %v", err)
	}
}

func TestWriteK8sResource_AutoFilename_Namespaced(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "mycm",
				"namespace": "myns",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}
	raw, err := u.MarshalJSON()
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	filename, err := lfs.WriteK8sResource("", manifest)
	if err != nil {
		t.Fatalf("WriteK8sResource error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	fmt.Println(s)
	if !strings.Contains(s, "apiVersion: v1") {
		t.Fatalf("missing apiVersion in file: %s", s)
	}
	if !strings.Contains(s, "kind: ConfigMap") {
		t.Fatalf("missing kind in file: %s", s)
	}
	if !strings.Contains(s, "name: mycm") {
		t.Fatalf("missing name in file: %s", s)
	}
	if !strings.Contains(s, "namespace: myns") {
		t.Fatalf("missing namespace in file: %s", s)
	}
	if !strings.Contains(s, "key: value") {
		t.Fatalf("missing data in file: %s", s)
	}
}

func TestWriteK8sResource_CustomFilename_NoNamespace(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "nonamespaced",
			},
			"data": map[string]interface{}{
				"a": "b",
			},
		},
	}
	manifest := runtime.RawExtension{Object: u}

	const custom = "custom-name.yaml"
	filename, err := lfs.WriteK8sResource(custom, manifest)
	if err != nil {
		t.Fatalf("WriteK8sResource error: %v", err)
	}
	if filename != custom {
		t.Fatalf("expected filename %q, got %q", custom, filename)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "name: nonamespaced") {
		t.Fatalf("missing name in file: %s", s)
	}
	if !strings.Contains(s, "a: b") {
		t.Fatalf("missing data in file: %s", s)
	}
	fmt.Println(s)
}
func TestWriteK8sResourceJQ_NoFilters_AutoFilename_Namespaced(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "jqcm",
				"namespace": "jqns",
			},
			"data": map[string]interface{}{
				"foo": "bar",
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	filename, err := lfs.WriteK8sResourceJQ("", manifest)
	if err != nil {
		t.Fatalf("WriteK8sResourceJQ error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "apiVersion: v1") {
		t.Fatalf("missing apiVersion in file: %s", s)
	}
	if !strings.Contains(s, "kind: ConfigMap") {
		t.Fatalf("missing kind in file: %s", s)
	}
	if !strings.Contains(s, "name: jqcm") {
		t.Fatalf("missing name in file: %s", s)
	}
	if !strings.Contains(s, "namespace: jqns") {
		t.Fatalf("missing namespace in file: %s", s)
	}
	if !strings.Contains(s, "foo: bar") {
		t.Fatalf("missing data in file: %s", s)
	}
}

func TestWriteK8sResourceJQ_WithJQ_NoOpFilter(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "secret-noop",
			},
			"stringData": map[string]interface{}{
				"k": "v",
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	// use the identity filter which should be a no-op
	filename, err := lfs.WriteK8sResourceJQ("", manifest, ".")
	if err != nil {
		t.Fatalf("WriteK8sResourceJQ error with jq filter: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "kind: Secret") {
		t.Fatalf("missing kind in file: %s", s)
	}
	if !strings.Contains(s, "name: secret-noop") {
		t.Fatalf("missing name in file: %s", s)
	}
	if !strings.Contains(s, "k: v") {
		t.Fatalf("missing data in file: %s", s)
	}
}

func TestWriteK8sResourceJQ_CustomFilename_NoNamespace(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "plain",
			},
			"data": map[string]interface{}{
				"x": "y",
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	const custom = "jq-custom.yaml"
	filename, err := lfs.WriteK8sResourceJQ(custom, manifest, ".")
	if err != nil {
		t.Fatalf("WriteK8sResourceJQ error: %v", err)
	}
	if filename != custom {
		t.Fatalf("expected filename %q, got %q", custom, filename)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "name: plain") {
		t.Fatalf("missing name in file: %s", s)
	}
	if !strings.Contains(s, "x: y") {
		t.Fatalf("missing data in file: %s", s)
	}
}

func TestWriteK8sResourceJQ_MissingFields_Error(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	// missing kind and apiVersion
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "bad",
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	if _, err := lfs.WriteK8sResourceJQ("", manifest); err == nil {
		t.Fatalf("expected error for resource missing required fields, got nil")
	}
}

func TestWriteK8sResourceJQ_InvalidJQFilter_Error(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "invalid-jq",
			},
			"data": map[string]interface{}{
				"foo": "bar",
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	// invalid jq filter
	if _, err := lfs.WriteK8sResourceJQ("", manifest, "invalid filter !!"); err == nil {
		t.Fatalf("expected error for invalid jq filter, got nil")
	}
}

func TestWriteK8sResourceJQ_RemoveFieldWithJQFilter(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "my-deploy",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "my-app",
					},
				},
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	// JQ filter to remove the spec.replicas field
	jqFilter := "del(.spec.replicas)"

	filename, err := lfs.WriteK8sResourceJQ("", manifest, jqFilter)
	if err != nil {
		t.Fatalf("WriteK8sResourceJQ error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	fmt.Println(s)
	if strings.Contains(s, "replicas") {
		t.Fatalf("expected replicas field to be removed, but found in file: %s", s)
	}
}

func TestWriteK8sResourceJQ_ModifyFieldWithJQFilter(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "modify-deploy",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": 2,
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	// JQ filter to change spec.replicas to 5
	jqFilter := ".spec.replicas = 5"

	filename, err := lfs.WriteK8sResourceJQ("", manifest, jqFilter)
	if err != nil {
		t.Fatalf("WriteK8sResourceJQ error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	fmt.Println(s)
	if !strings.Contains(s, "replicas: 5") {
		t.Fatalf("expected replicas field to be modified to 5, but got file: %s", s)
	}
}

func TestWriteK8sResourceJQ_MultipleJQFilters(t *testing.T) {
	base := t.TempDir()
	lfs, err := NewLocalFS(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if err := lfs.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	}()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "multi-deploy",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "app-container",
								"image": "old-image:v1",
							},
						},
					},
				},
			},
		},
	}
	raw, err := u.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal json error: %v", err)
	}
	manifest := runtime.RawExtension{Raw: raw, Object: u}

	// JQ filters to change spec.replicas to 4 and update container image
	jqFilters := []string{
		".spec.replicas = 4",
		".spec.template.spec.containers[0].image = \"new-image:v2\"",
	}

	filename, err := lfs.WriteK8sResourceJQ("", manifest, jqFilters...)
	if err != nil {
		t.Fatalf("WriteK8sResourceJQ error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(lfs.tmp, filename))
	if err != nil {
		t.Fatalf("read written file error: %v", err)
	}
	s := string(content)
	fmt.Println(s)
	if !strings.Contains(s, "replicas: 4") {
		t.Fatalf("expected replicas field to be modified to 4, but got file: %s", s)
	}
	if !strings.Contains(s, "image: new-image:v2") {
		t.Fatalf("expected container image to be updated, but got file: %s", s)
	}
}
