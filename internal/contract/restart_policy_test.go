package contract

import (
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// The operator requires manual startup, including after host/Docker restarts.
// 操作者要求仅手动启动，宿主机/Docker 重启后也不自动拉起。
func TestComposeRequiresManualContainerStart(t *testing.T) {
	var compose struct {
		Services map[string]struct {
			Restart any `yaml:"restart"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "deploy/docker/docker-compose.yml")), &compose); err != nil {
		t.Fatal(err)
	}
	service, ok := compose.Services["gsm-system"]
	if !ok {
		t.Fatal("GSM Compose service missing")
	}
	policy, isString := service.Restart.(string)
	if !isString || policy != "no" {
		t.Fatalf("GSM restart policy = %#v, want string no (manual start only)", service.Restart)
	}
}
