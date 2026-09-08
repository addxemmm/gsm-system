package contract

import (
	"path/filepath"
	"strings"
	"testing"
)

// Receive support must never imply that the CLI can submit arbitrary Unicode.
// 接收端支持不等于 CLI 支持 Unicode 发送，文档必须明确区分。
func TestSMSUnicodeReceiveDocumentationContract(t *testing.T) {
	for _, path := range []string{"docs/API.md", "docs/api/openapi.yaml", "postman/gsm-system.postman_collection.json"} {
		content := string(mustRead(t, filepath.Join(root(t), filepath.FromSlash(path))))
		for _, marker := range []string{"0x08", "0x18..0x1b", "surrogate", "ASCII"} {
			if !strings.Contains(content, marker) {
				t.Errorf("%s missing Unicode receive limit %q", path, marker)
			}
		}
	}
}
