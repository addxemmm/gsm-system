package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}
}

type fixtureOptions struct {
	tokenStatus, manifestStatus, configStatus      int
	config, manifest, manifestHeader, configHeader string
	wrongConfigDigest                              bool
}

const validConfig = `{"os":"linux","architecture":"amd64","config":{"Labels":{"org.opencontainers.image.version":"2.1.0","org.opencontainers.image.revision":"0123456789ab"}}}`

func fixture(t *testing.T, o fixtureOptions) *http.Client {
	t.Helper()
	if o.config == "" {
		o.config = validConfig
	}
	digest := hash([]byte(o.config))
	if o.wrongConfigDigest {
		digest = "sha256:" + strings.Repeat("0", 64)
	}
	if o.manifest == "" {
		o.manifest = fmt.Sprintf(`{"schemaVersion":2,"config":{"digest":%q}}`, digest)
	}
	client := newClient()
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "auth.docker.io":
			user, password, ok := r.BasicAuth()
			if !ok || user != "fixture-user" || password != "fixture-password" {
				t.Fatal("missing basic auth")
			}
			if r.URL.Query().Get("scope") != "repository:addxemmm/gsm-system:pull" {
				t.Fatal("wrong token scope")
			}
			status := o.tokenStatus
			if status == 0 {
				status = 200
			}
			return response(status, `{"token":"fixture-bearer"}`, nil), nil
		case strings.Contains(r.URL.Path, "/manifests/"):
			if r.Header.Get("Authorization") != "Bearer fixture-bearer" || r.Header.Get("Accept") != manifestAccept {
				t.Fatal("missing manifest auth or accept")
			}
			status := o.manifestStatus
			if status == 0 {
				status = 200
			}
			return response(status, o.manifest, http.Header{"Docker-Content-Digest": {o.manifestHeader}}), nil
		case strings.Contains(r.URL.Path, "/blobs/"):
			status := o.configStatus
			if status == 0 {
				status = 200
			}
			return response(status, o.config, http.Header{"Docker-Content-Digest": {o.configHeader}}), nil
		default:
			t.Fatal("unexpected request")
			return nil, fmt.Errorf("unexpected request")
		}
	})
	return client
}

func TestInspectPresentAndJSONContract(t *testing.T) {
	value, err := inspect(fixture(t, fixtureOptions{}), "addxemmm/gsm-system", "2.1.0", "fixture-user", "fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if value.State != "present" || value.Version != "2.1.0" || value.Revision != "0123456789ab" || value.ConfigDigest != hash([]byte(validConfig)) || !digestRE.MatchString(value.Digest) {
		t.Fatalf("unexpected result: %+v", value)
	}
	encoded, _ := json.Marshal(value)
	var decoded map[string]string
	if json.Unmarshal(encoded, &decoded) != nil || len(decoded) != 5 || decoded["config_digest"] == "" {
		t.Fatal("JSON contract changed")
	}
}

func TestOnlyManifest404IsAbsent(t *testing.T) {
	for _, stage := range []string{"token", "manifest", "config"} {
		for _, status := range []int{200, 201, 400, 401, 403, 404, 429, 500, 503} {
			t.Run(fmt.Sprintf("%s-%d", stage, status), func(t *testing.T) {
				o := fixtureOptions{}
				switch stage {
				case "token":
					o.tokenStatus = status
				case "manifest":
					o.manifestStatus = status
				case "config":
					o.configStatus = status
				}
				value, err := inspect(fixture(t, o), "addxemmm/gsm-system", "2.1.0", "fixture-user", "fixture-password")
				if stage == "manifest" && status == 404 {
					if err != nil || value.State != "absent" {
						t.Fatal("manifest404 did not report absent")
					}
					encoded, _ := json.Marshal(value)
					if string(encoded) != `{"state":"absent"}` {
						t.Fatal("absent JSON contract changed")
					}
				} else if status == 200 {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("non-success treated as successful absence")
				}
			})
		}
	}
}

func TestInvalidMetadata(t *testing.T) {
	for name, o := range map[string]fixtureOptions{
		"manifest-digest": {manifestHeader: "sha256:" + strings.Repeat("0", 64)},
		"config-header":   {configHeader: "sha256:" + strings.Repeat("0", 64)},
		"config-digest":   {wrongConfigDigest: true},
		"index":           {manifest: `{"schemaVersion":2,"manifests":[]}`},
		"index-null":      {manifest: `{"schemaVersion":2,"manifests":null}`},
		"malformed":       {manifest: `not-json`},
		"manifest-array":  {manifest: `[]`},
		"config-array":    {config: `[]`},
		"architecture":    {config: strings.Replace(validConfig, "amd64", "arm64", 1)},
		"os":              {config: strings.Replace(validConfig, "linux", "windows", 1)},
		"version":         {config: strings.Replace(validConfig, "2.1.0", "2.1", 1)},
		"revision":        {config: strings.Replace(validConfig, "0123456789ab", "full-40-or-invalid", 1)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := inspect(fixture(t, o), "addxemmm/gsm-system", "2.1.0", "fixture-user", "fixture-password"); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
}

func TestIdentityAndCredentialsRejectedBeforeRequest(t *testing.T) {
	client := newClient()
	client.Transport = transportFunc(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected request"); return nil, nil })
	for _, v := range [][4]string{
		{"../other", "2.1.0", "u", "p"}, {"addxemmm/gsm-system", "tag/escape", "u", "p"},
		{"addxemmm/gsm-system", "2.1.0", "", "p"}, {"addxemmm/gsm-system", "2.1.0", "u", ""},
	} {
		if _, err := inspect(client, v[0], v[1], v[2], v[3]); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}

func TestRedirectStripsAuthAcrossHostsAndNeverRestores(t *testing.T) {
	client := newClient()
	step := 0
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		step++
		switch step {
		case 1:
			if r.Header.Get("Authorization") != "Bearer fixture" {
				t.Fatal("initial auth missing")
			}
			return response(307, "", http.Header{"Location": {"https://cdn.example/blob"}}), nil
		case 2:
			if r.Header.Get("Authorization") != "" {
				t.Fatal("cross-host auth leaked")
			}
			return response(307, "", http.Header{"Location": {"https://registry.example/returned"}}), nil
		case 3:
			if r.Header.Get("Authorization") != "" {
				t.Fatal("auth restored after cross-host redirect")
			}
			return response(200, "{}", nil), nil
		default:
			t.Fatal("redirect loop")
			return nil, nil
		}
	})
	if _, _, _, err := request(client, "https://registry.example/start", "Bearer fixture", "", false); err != nil {
		t.Fatal(err)
	}
}

func TestRejectDowngradeUserinfoAndRedirectLoops(t *testing.T) {
	for _, target := range []string{"http://registry.example/blob", "https://user:password@registry.example/blob", "https://registry.example/loop"} {
		client := newClient()
		client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
			return response(307, "", http.Header{"Location": {target}}), nil
		})
		if _, _, _, err := request(client, "https://registry.example/start", "Bearer fixture", "", false); err == nil {
			t.Fatal("unsafe redirect accepted")
		}
	}
}

func TestMetadataLimitAndTimeout(t *testing.T) {
	client := newClient()
	if client.Timeout != 60*time.Second {
		t.Fatal("missing bounded request timeout")
	}
	client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return response(200, strings.Repeat("x", maxMetadata+1), nil), nil
	})
	if _, _, _, err := request(client, "https://registry.example/blob", "Bearer fixture", "", false); err == nil {
		t.Fatal("oversized metadata accepted")
	}
	if err := selfTest(); err != nil {
		t.Fatal(err)
	}
}
