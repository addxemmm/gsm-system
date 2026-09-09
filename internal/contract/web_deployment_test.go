package contract

import (
	"os"
	"strings"
	"testing"
)

func TestWebOnlyDefaultAndExplicitAPIOverlay(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	base := read("deploy/docker/docker-compose.yml")
	if !strings.Contains(base, `${GSM_WEB_PORT:-18082}:18082/tcp`) ||
		!strings.Contains(base, `GSM_WEB_LISTEN: ":18082"`) ||
		!strings.Contains(base, `GSM_LISTEN: "127.0.0.1:8082"`) ||
		strings.Contains(base, `:8082/tcp`) {
		t.Fatal("default must publish Web only and bind standalone API to loopback")
	}
	overlay := read("deploy/docker/docker-compose.api.yml")
	if !strings.Contains(overlay, `${GSM_API_PORT:-8082}:8082/tcp`) ||
		!strings.Contains(overlay, `GSM_LISTEN: ":8082"`) {
		t.Fatal("explicit API overlay must publish and listen on the API port")
	}
	if !strings.Contains(read(".env.example"), "GSM_EXPOSE_API=false") {
		t.Fatal("example must default to no standalone API publishing")
	}
	for path, expected := range map[string]string{
		".env.example":                                        "GSM_WEB_PORT=18082",
		"configs/app.yaml.example":                            `web_listen: ":18082"`,
		"deploy/docker/Dockerfile":                            "EXPOSE 18082 8082",
		"postman/gsm-system.postman_collection.json":          "http://127.0.0.1:18082",
		"postman/gsm-system.postman_environment.example.json": "http://HOST:18082",
	} {
		if !strings.Contains(read(path), expected) {
			t.Errorf("%s must contain %q", path, expected)
		}
	}
}
