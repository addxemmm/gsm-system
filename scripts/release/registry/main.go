// Read-only Docker Hub inspection / Docker Hub 只读镜像校验。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const maxMetadata = 16 * 1024 * 1024
const manifestAccept = "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"

var repositoryRE = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*/[a-z0-9]+([._-][a-z0-9]+)*$`)
var tagRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
var digestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var versionRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var revisionRE = regexp.MustCompile(`^[0-9a-f]{12}$`)

type result struct {
	State        string `json:"state"`
	Digest       string `json:"digest,omitempty"`
	ConfigDigest string `json:"config_digest,omitempty"`
	Version      string `json:"version,omitempty"`
	Revision     string `json:"revision,omitempty"`
}

func safeRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.User != nil {
		return errors.New("invalid registry redirect")
	}
	// Once a redirect crosses a host boundary, never restore its original auth.
	// 重定向跨主机后不恢复原始认证头，即使后续跳回原主机。
	for _, previous := range via {
		if previous.URL.Host != req.URL.Host {
			req.Header.Del("Authorization")
		}
	}
	return nil
}

func newClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second, CheckRedirect: safeRedirect}
}

func statusState(status int, allowAbsent bool) (string, error) {
	if status == http.StatusOK {
		return "present", nil
	}
	if status == http.StatusNotFound && allowAbsent {
		return "absent", nil
	}
	return "", fmt.Errorf("registry HTTP %d; absence was not established", status)
}

func request(client *http.Client, address, authorization, accept string, allowAbsent bool) ([]byte, http.Header, bool, error) {
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil || req.URL.Scheme != "https" || req.URL.User != nil {
		return nil, nil, false, errors.New("invalid registry URL")
	}
	req.Header.Set("Authorization", authorization)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, nil, false, errors.New("registry network request failed")
	}
	defer response.Body.Close()
	state, err := statusState(response.StatusCode, allowAbsent)
	if err != nil {
		return nil, nil, false, err
	}
	if state == "absent" {
		return nil, response.Header, true, nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxMetadata+1))
	if err != nil || len(data) > maxMetadata {
		return nil, nil, false, errors.New("invalid or oversized registry metadata")
	}
	return data, response.Header, false, nil
}

func hash(data []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(data)) }

func token(client *http.Client, repository, username, password string) (string, error) {
	if username == "" || password == "" {
		return "", errors.New("DOCKERHUB_USERNAME and DOCKERHUB_TOKEN are required")
	}
	q := url.Values{"service": {"registry.docker.io"}, "scope": {"repository:" + repository + ":pull"}}
	req, _ := http.NewRequest(http.MethodGet, "https://auth.docker.io/token?"+q.Encode(), nil)
	req.SetBasicAuth(username, password)
	data, _, _, err := request(client, req.URL.String(), req.Header.Get("Authorization"), "", false)
	if err != nil {
		return "", err
	}
	var auth struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(data, &auth) != nil || strings.TrimSpace(auth.Token) == "" {
		return "", errors.New("registry token missing or invalid")
	}
	return auth.Token, nil
}

func inspect(client *http.Client, repository, tag, username, password string) (result, error) {
	if !repositoryRE.MatchString(repository) || len(repository) > 200 || !tagRE.MatchString(tag) {
		return result{}, errors.New("invalid Docker Hub repository or tag")
	}
	bearer, err := token(client, repository, username, password)
	if err != nil {
		return result{}, err
	}
	base := "https://registry-1.docker.io/v2/" + repository + "/"
	manifestData, headers, absent, err := request(client, base+"manifests/"+url.PathEscape(tag), "Bearer "+bearer, manifestAccept, true)
	if err != nil {
		return result{}, err
	}
	if absent {
		return result{State: "absent"}, nil
	}
	digest := hash(manifestData)
	if header := headers.Get("Docker-Content-Digest"); header != "" && header != digest {
		return result{}, errors.New("manifest header/content digest mismatch")
	}
	var manifest struct {
		SchemaVersion int             `json:"schemaVersion"`
		Manifests     json.RawMessage `json:"manifests"`
		MediaType     string          `json:"mediaType"`
		Config        struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	if json.Unmarshal(manifestData, &manifest) != nil || manifest.SchemaVersion != 2 || manifest.Manifests != nil || !digestRE.MatchString(manifest.Config.Digest) {
		return result{}, errors.New("only valid single-platform image manifests are accepted")
	}
	if manifest.MediaType != "" && manifest.MediaType != "application/vnd.oci.image.manifest.v1+json" && manifest.MediaType != "application/vnd.docker.distribution.manifest.v2+json" {
		return result{}, errors.New("unsupported image manifest media type")
	}
	configData, configHeaders, _, err := request(client, base+"blobs/"+manifest.Config.Digest, "Bearer "+bearer, "", false)
	if err != nil {
		return result{}, err
	}
	if hash(configData) != manifest.Config.Digest {
		return result{}, errors.New("image config content digest mismatch")
	}
	if header := configHeaders.Get("Docker-Content-Digest"); header != "" && header != manifest.Config.Digest {
		return result{}, errors.New("image config header digest mismatch")
	}
	var config struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Config       struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if json.Unmarshal(configData, &config) != nil || config.OS != "linux" || config.Architecture != "amd64" {
		return result{}, errors.New("release image must be linux/amd64")
	}
	version, revision := config.Config.Labels["org.opencontainers.image.version"], config.Config.Labels["org.opencontainers.image.revision"]
	if !versionRE.MatchString(version) || !revisionRE.MatchString(revision) {
		return result{}, errors.New("invalid image version or revision label")
	}
	return result{State: "present", Digest: digest, ConfigDigest: manifest.Config.Digest, Version: version, Revision: revision}, nil
}

func selfTest() error {
	for _, status := range []int{0, 200, 201, 400, 401, 403, 404, 429, 500, 503} {
		for _, allow := range []bool{false, true} {
			state, err := statusState(status, allow)
			if status == 200 {
				if err != nil || state != "present" {
					return errors.New("status self-test failed")
				}
			} else if status == 404 && allow {
				if err != nil || state != "absent" {
					return errors.New("absence self-test failed")
				}
			} else if err == nil {
				return errors.New("fail-closed self-test failed")
			}
		}
	}
	if !repositoryRE.MatchString("addxemmm/gsm-system") || !tagRE.MatchString("2.1.0") || !versionRE.MatchString("2.1.0") || versionRE.MatchString("2.1") {
		return errors.New("identity self-test failed")
	}
	return nil
}

func run(args []string) error {
	if len(args) == 1 && args[0] == "self-test" {
		if err := selfTest(); err != nil {
			return err
		}
		fmt.Println("PASS ci-release registry status is fail-closed / registry 状态仅 manifest 404 视为不存在")
		return nil
	}
	if len(args) != 3 || args[0] != "inspect" {
		return errors.New("usage: scripts/ci-release.sh inspect REPOSITORY TAG | self-test")
	}
	value, err := inspect(newClient(), args[1], args[2], os.Getenv("DOCKERHUB_USERNAME"), os.Getenv("DOCKERHUB_TOKEN"))
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(value)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Release registry check stopped / 发布仓库校验停止:", err)
		os.Exit(1)
	}
}
