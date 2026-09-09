// Legacy/manual server promotion and registry verification utility.
// Normal releases use .github/workflows/release.yml; this never builds/deploys.
// 手动服务器发布/摘要校验备用工具；正常发版使用 release 工作流。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var (
	versionRE    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	revisionRE   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestRE     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	repositoryRE = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*/[a-z0-9]+([._-][a-z0-9]+)*$`)
)

type options struct {
	mode, version, revision, repository, digest, notes string
	accepted, privateReviewed, immutable               bool
}

func identity(o options) (string, error) {
	if !versionRE.MatchString(o.version) || !revisionRE.MatchString(o.revision) ||
		!repositoryRE.MatchString(o.repository) || len(o.repository) > 200 {
		return "", errors.New("expected x.y.z version, full lowercase Git SHA, and Docker Hub namespace/name")
	}
	tag := "v" + o.version + "-" + o.revision[:12]
	if len(tag) > 128 {
		return "", errors.New("image tag exceeds 128 characters")
	}
	return tag, nil
}

func command(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

func checkSource(o options) error {
	b, err := os.ReadFile("VERSION")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) != o.version {
		return errors.New("VERSION mismatch")
	}
	// The deployment sync uses git archive, not a Git checkout. Match its exact
	// 12-character marker; the workflow independently checks the full commit/tag.
	// 服务器由 git archive 同步；核对部署标记，完整提交与 tag 由云端独立校验。
	if o.mode == "publish" {
		marker, err := os.ReadFile(".release-revision")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(marker)) != o.revision[:12] {
			return errors.New("deployed .release-revision mismatch")
		}
		return nil
	}
	head, err := command("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != o.revision {
		return errors.New("checkout HEAD differs from revision")
	}
	status, err := command("git", "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if status != "" {
		return errors.New("release requires a clean committed checkout")
	}
	return nil
}

type imageConfig struct {
	ID           string `json:"Id"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Config       struct {
		Labels map[string]string `json:"Labels"`
	} `json:"config"`
}

func checkLabels(c imageConfig, o options) error {
	if c.Config.Labels["org.opencontainers.image.version"] != o.version ||
		c.Config.Labels["org.opencontainers.image.revision"] != o.revision[:12] {
		return errors.New("OCI version/revision mismatch")
	}
	return nil
}

func checkServer(o options) (string, error) {
	b, err := command("docker", "inspect", "gsmsystem-uhd4")
	if err != nil {
		return "", err
	}
	var containers []struct {
		Image  string
		Config struct{ Labels map[string]string }
		State  struct {
			Running bool
			Health  struct{ Status string }
		}
	}
	if err = json.Unmarshal([]byte(b), &containers); err != nil {
		return "", err
	}
	if len(containers) != 1 {
		return "", errors.New("expected one managed GSM container")
	}
	c := containers[0]
	if !c.State.Running || c.State.Health.Status != "healthy" || c.Config.Labels["com.gsm-system.managed"] != "true" {
		return "", errors.New("managed GSM container must be running and healthy")
	}
	b, err = command("docker", "image", "inspect", "gsm-system:2.1")
	if err != nil {
		return "", err
	}
	var images []imageConfig
	if err = json.Unmarshal([]byte(b), &images); err != nil {
		return "", err
	}
	if len(images) != 1 || images[0].ID != c.Image || !digestRE.MatchString(c.Image) {
		return "", errors.New("running container differs from gsm-system:2.1")
	}
	if err = checkLabels(images[0], o); err != nil {
		return "", err
	}
	b, err = command("docker", "exec", "gsmsystem-uhd4", "/usr/local/bin/gsm-system", "--version")
	if err != nil {
		return "", err
	}
	if b != fmt.Sprintf("gsm-system %s (%s)", o.version, o.revision[:12]) {
		return "", errors.New("running binary revision mismatch")
	}
	_, err = command("docker", "exec", "gsmsystem-uhd4", "/usr/local/bin/gsm-system", "--healthcheck")
	return c.Image, err
}

type registry struct {
	client            *http.Client
	repository, token string
}

const maxMetadata = 16 * 1024 * 1024
const media = "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"

func newRegistry(repository string) (*registry, error) {
	r := &registry{repository: repository, client: &http.Client{Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Scheme != "https" {
				return errors.New("invalid registry redirect")
			}
			if req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
			}
			return nil
		}}}
	q := url.Values{"service": {"registry.docker.io"}, "scope": {"repository:" + repository + ":pull"}}
	req, err := http.NewRequest("GET", "https://auth.docker.io/token?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	user, token := os.Getenv("DOCKERHUB_USER"), os.Getenv("DOCKERHUB_TOKEN")
	if (user == "") != (token == "") {
		return nil, errors.New("set both DOCKERHUB_USER and DOCKERHUB_TOKEN")
	}
	if token != "" {
		req.SetBasicAuth(user, token)
	}
	b, _, err := r.fetch(req)
	if err != nil {
		return nil, err
	}
	var auth struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(b, &auth); err != nil {
		return nil, err
	}
	if auth.Token == "" {
		return nil, errors.New("registry token missing")
	}
	r.token = auth.Token
	return r, nil
}

func (r *registry) fetch(req *http.Request) ([]byte, int, error) {
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("registry HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadata+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(b) > maxMetadata {
		return nil, resp.StatusCode, errors.New("registry metadata exceeds size limit")
	}
	return b, resp.StatusCode, nil
}

func (r *registry) get(kind, ref string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", "https://registry-1.docker.io/v2/"+r.repository+"/"+kind+"/"+ref, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", media)
	return r.fetch(req)
}

func hash(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

type metadataGetter interface {
	get(string, string) ([]byte, int, error)
}

func ensureTagAbsent(r metadataGetter, tag string) error {
	_, status, err := r.get("manifests", tag)
	if status == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("remote image tag already exists; overwrite is disabled")
}

func verifyRegistry(r metadataGetter, tag string, o options) (string, error) {
	if !digestRE.MatchString(o.digest) {
		return "", errors.New("invalid sha256 digest")
	}
	raw, _, err := r.get("manifests", o.digest)
	if err != nil {
		return "", err
	}
	if hash(raw) != o.digest {
		return "", errors.New("manifest content digest mismatch")
	}
	tagRaw, _, err := r.get("manifests", tag)
	if err != nil {
		return "", err
	}
	if hash(tagRaw) != o.digest {
		return "", errors.New("remote tag differs from digest")
	}
	var manifest struct {
		SchemaVersion int `json:"schemaVersion"`
		Config        struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Manifests []json.RawMessage `json:"manifests"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return "", err
	}
	if manifest.SchemaVersion != 2 || len(manifest.Manifests) != 0 || !digestRE.MatchString(manifest.Config.Digest) {
		return "", errors.New("only tested single-platform image manifests are supported")
	}
	b, _, err := r.get("blobs", manifest.Config.Digest)
	if err != nil {
		return "", err
	}
	if hash(b) != manifest.Config.Digest {
		return "", errors.New("config digest mismatch")
	}
	var config imageConfig
	if err = json.Unmarshal(b, &config); err != nil {
		return "", err
	}
	if config.OS != "linux" || config.Architecture != "amd64" {
		return "", errors.New("image must be linux/amd64")
	}
	if err = checkLabels(config, o); err != nil {
		return "", err
	}
	return manifest.Config.Digest, nil
}

func releaseNotes(o options, tag string) string {
	return fmt.Sprintf(`# gsm-system v%s

## Changes / 变更
- Exact source / 精确源码: `+"`%s`"+`
- Bilingual changes and known issues: commit history and docs/RELEASE-2.1.md at this revision.
  双语变更、已知问题见此提交的历史及 docs/RELEASE-2.1.md。

## Artifact / 发布产物
- Docker Hub: `+"`docker.io/%s:%s`"+`
- Immutable reference / 内容寻址: `+"`docker.io/%s@%s`"+`
- Platform / 平台: linux/amd64; runtime alias / 运行别名: gsm-system:2.1.

## Validation / 验证
- Workflow source tests and remote manifest/config digest + OCI checks passed.
  工作流源码测试与远端 manifest/config 摘要及版本校验通过。
- These checks do not prove handset RF, SMS, voice or GPRS acceptance.
  以上检查不证明手机射频、短信、语音或 GPRS 实机业务验收通过；云端未重跑硬件测试。

## Rollback / 回滚
Use the previous release digest through the server deployment gates; preserve business data.
通过服务器部署验收流程使用上一发布 digest 回滚，保留业务数据卷。
`, o.version, o.revision, o.repository, tag, o.repository, o.digest)
}

func execute(o options) error {
	tag, err := identity(o)
	if err != nil {
		return err
	}
	ref := "docker.io/" + o.repository + ":" + tag
	if o.mode == "plan" {
		fmt.Printf("Plan only / 仅计划: %s; GitHub tag v%s; runtime gsm-system:2.1\n", ref, o.version)
		return nil
	}
	if o.mode != "publish" && o.mode != "verify" {
		return errors.New("mode must be plan, publish or verify")
	}
	if err = checkSource(o); err != nil {
		return err
	}
	if o.mode == "verify" && !digestRE.MatchString(o.digest) {
		return errors.New("verify requires valid --digest")
	}
	if o.mode == "publish" && (!o.accepted || !o.privateReviewed || !o.immutable || runtime.GOOS != "linux") {
		return errors.New("publish requires Linux SDR server plus acceptance/private-repository/immutable-tag confirmations")
	}
	r, err := newRegistry(o.repository)
	if err != nil {
		return err
	}
	if o.mode == "publish" {
		imageID, err := checkServer(o)
		if err != nil {
			return err
		}
		// Only an explicit 404 means absent; auth/rate-limit/network errors fail closed.
		// 只有明确 404 才视作不存在；认证、限流、网络错误直接停止。
		if err = ensureTagAbsent(r, tag); err != nil {
			return err
		}
		if _, err = command("docker", "tag", imageID, ref); err != nil {
			return err
		}
		push := exec.Command("docker", "push", ref)
		push.Stdout, push.Stderr = os.Stdout, os.Stderr
		if err = push.Run(); err != nil {
			return err
		}
		r, err = newRegistry(o.repository)
		if err != nil {
			return err
		}
		raw, _, err := r.get("manifests", tag)
		if err != nil {
			return err
		}
		o.digest = hash(raw)
		configID, err := verifyRegistry(r, tag, o)
		if err != nil {
			return err
		}
		if configID != imageID {
			return errors.New("published image differs from accepted local image")
		}
	} else if _, err = verifyRegistry(r, tag, o); err != nil {
		return err
	}
	if o.notes != "" {
		f, err := os.OpenFile(o.notes, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(f, releaseNotes(o, tag))
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"version": o.version, "revision": o.revision,
		"release_tag": "v" + o.version, "image": ref, "digest": o.digest})
}

func main() {
	var o options
	flag.StringVar(&o.mode, "mode", "plan", "plan (default), publish (server only), verify")
	flag.StringVar(&o.version, "version", "", "source VERSION x.y.z")
	flag.StringVar(&o.revision, "revision", "", "full source Git SHA")
	flag.StringVar(&o.repository, "repository", "", "Docker Hub namespace/name")
	flag.StringVar(&o.digest, "digest", "", "remote manifest sha256 digest")
	flag.StringVar(&o.notes, "notes", "", "new notes file, never overwritten")
	flag.BoolVar(&o.accepted, "acceptance-passed", false, "operator confirms server/hardware acceptance")
	flag.BoolVar(&o.privateReviewed, "private-repo-reviewed", false, "operator confirms private Hub repo and image-content review")
	flag.BoolVar(&o.immutable, "immutable-tags-confirmed", false, "operator confirms Hub immutable tags")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if err := execute(o); err != nil {
		fmt.Fprintln(os.Stderr, "Release stopped / 发布停止:", err)
		os.Exit(1)
	}
}
