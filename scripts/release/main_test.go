package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func sampleOptions() options {
	return options{mode: "plan", version: "2.1.0", revision: strings.Repeat("a", 40), repository: "example/gsm-system"}
}

func TestIdentity(t *testing.T) {
	o := sampleOptions()
	tag, err := identity(o)
	if err != nil || tag != "v2.1.0-aaaaaaaaaaaa" {
		t.Fatalf("%s %v", tag, err)
	}
	for _, value := range []string{"2.1", "latest", "02.1.0", "2.1.0;echo x", "2.1.0\n"} {
		bad := o
		bad.version = value
		if _, err := identity(bad); err == nil {
			t.Errorf("accepted version %q", value)
		}
	}
	for _, value := range []string{"host/x/y", "user/repo:latest", "https://host/repo", "../repo", "user/repo\n", "UPPER/repo", "user/$(id)"} {
		bad := o
		bad.repository = value
		if _, err := identity(bad); err == nil {
			t.Errorf("accepted repository %q", value)
		}
	}
	for _, value := range []string{"HEAD", strings.Repeat("A", 40), strings.Repeat("a", 12), strings.Repeat("a", 40) + "\n"} {
		bad := o
		bad.revision = value
		if _, err := identity(bad); err == nil {
			t.Errorf("accepted revision %q", value)
		}
	}
}

func TestPlanNeedsNoGitDockerOrNetwork(t *testing.T) {
	t.Setenv("PATH", "")
	if err := execute(sampleOptions()); err != nil {
		t.Fatal(err)
	}
}

type fakeRegistry map[string][]byte

func (r fakeRegistry) get(kind, ref string) ([]byte, int, error) {
	if data, ok := r[kind+"/"+ref]; ok {
		return data, http.StatusOK, nil
	}
	return nil, http.StatusNotFound, errors.New("missing")
}

func registryFixture(o options) (fakeRegistry, options, string) {
	config, _ := json.Marshal(map[string]any{"os": "linux", "architecture": "amd64", "config": map[string]any{
		"Labels": map[string]string{"org.opencontainers.image.version": o.version, "org.opencontainers.image.revision": o.revision[:12]}}})
	configID := hash(config)
	manifest, _ := json.Marshal(map[string]any{"schemaVersion": 2, "config": map[string]string{"digest": configID}})
	o.digest = hash(manifest)
	tag, _ := identity(o)
	return fakeRegistry{"manifests/" + o.digest: manifest, "manifests/" + tag: manifest, "blobs/" + configID: config}, o, configID
}

func TestRegistryContentAndLabels(t *testing.T) {
	r, o, configID := registryFixture(sampleOptions())
	tag, _ := identity(o)
	got, err := verifyRegistry(r, tag, o)
	if err != nil || got != configID {
		t.Fatalf("%s %v", got, err)
	}
	for _, key := range []string{"manifests/" + o.digest, "manifests/" + tag, "blobs/" + configID} {
		original := r[key]
		r[key] = []byte(`{}`)
		if _, err := verifyRegistry(r, tag, o); err == nil {
			t.Errorf("accepted tampered %s", key)
		}
		r[key] = original
	}
	bad := o
	bad.version = "2.1.1"
	if _, err := verifyRegistry(r, tag, bad); err == nil {
		t.Fatal("accepted wrong OCI version")
	}
	bad = o
	bad.revision = strings.Repeat("b", 40)
	if _, err := verifyRegistry(r, tag, bad); err == nil {
		t.Fatal("accepted wrong OCI revision")
	}
	bad = o
	bad.digest = "sha256:no"
	if _, err := verifyRegistry(r, tag, bad); err == nil {
		t.Fatal("accepted malformed digest")
	}
}

func TestMultiPlatformRejected(t *testing.T) {
	o := sampleOptions()
	tag, _ := identity(o)
	raw := []byte(`{"schemaVersion":2,"manifests":[{}]}`)
	o.digest = hash(raw)
	r := fakeRegistry{"manifests/" + o.digest: raw, "manifests/" + tag: raw}
	if _, err := verifyRegistry(r, tag, o); err == nil {
		t.Fatal("accepted untested image index")
	}
}

type statusRegistry int

func (r statusRegistry) get(string, string) ([]byte, int, error) {
	if int(r) == http.StatusOK {
		return []byte(`{}`), int(r), nil
	}
	return nil, int(r), errors.New("registry failure")
}

func TestTagExistenceFailsClosed(t *testing.T) {
	for _, status := range []int{0, 200, 401, 403, 404, 429, 500, 503} {
		err := ensureTagAbsent(statusRegistry(status), "v2.1.0-aaaaaaaaaaaa")
		if (err == nil) != (status == 404) {
			t.Errorf("status %d: %v", status, err)
		}
	}
}

func TestRegistryErrorsStopVerification(t *testing.T) {
	o := sampleOptions()
	o.digest = "sha256:" + strings.Repeat("a", 64)
	for _, status := range []int{0, 401, 403, 404, 429, 500} {
		if _, err := verifyRegistry(statusRegistry(status), "tag", o); err == nil {
			t.Errorf("accepted status %d", status)
		}
	}
}

func TestNotesKeepRuntimeVersionAndDigest(t *testing.T) {
	o := sampleOptions()
	o.digest = "sha256:" + strings.Repeat("b", 64)
	tag, _ := identity(o)
	notes := releaseNotes(o, tag)
	for _, want := range []string{o.digest, o.revision, "gsm-system:2.1", "变更", "云端未重跑硬件测试"} {
		if !strings.Contains(notes, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(notes, ":latest") {
		t.Fatal("latest must not be used")
	}
}

func TestWorkflowBuildsTestsAndPublishesTaggedSource(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On          map[string]any    `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			RunsOn string `yaml:"runs-on"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &workflow); err != nil {
		t.Fatal(err)
	}
	if len(workflow.On) != 2 || workflow.On["workflow_dispatch"] == nil || workflow.On["push"] == nil {
		t.Fatal("release must support tag push and explicit dry-run/redispatch")
	}
	if workflow.Permissions["contents"] != "read" {
		t.Fatal("default permission must remain read-only")
	}
	for name, job := range workflow.Jobs {
		if job.RunsOn != "ubuntu-latest" {
			t.Errorf("%s unexpectedly uses a different runner", name)
		}
	}
	for _, required := range []string{
		"git merge-base --is-ancestor", "docker/build-push-action@", "load: true", "test_callerid_image.sh gsm-system:2.1",
		"test_image.sh gsm-system:2.1", "test_sms_image.sh gsm-system:2.1",
		"test_presets_image.sh gsm-system:2.1", "test_web_image.sh gsm-system:2.1",
		"docker push \"$version_ref\"", "docker push \"$stable_ref\"", "gh release create",
		"Physical GPRS handset", "scripts/release/source_bundle.sh",
	} {
		if !strings.Contains(string(b), required) {
			t.Errorf("workflow lacks %s", required)
		}
	}
	for _, forbidden := range []string{"docker commit", "docker export", "git tag ", "git push ", "--clobber", ":latest"} {
		if strings.Contains(string(b), forbidden) {
			t.Errorf("workflow contains %s", forbidden)
		}
	}
}
