package contract

import (
	"os"
	"strings"
	"testing"
)

func TestVoiceDiagnosticRoutesAndReservedNumberDocs(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	main := read("gsmsystem/asterisk/extensions.conf")
	if !strings.Contains(main, "#include extensions-gsm-diagnostics.conf") || strings.Contains(main, "#include extensions-range-test.conf") {
		t.Fatal("only the bounded diagnostic routes should be included")
	}
	dialplan := read("gsmsystem/asterisk/extensions-gsm-diagnostics.conf")
	for _, route := range []string{"[phones](+)", "[default](+)", "[gsm-diagnostics]", "Echo()", "Milliwatt()", "TIMEOUT(absolute)=60", "TIMEOUT(absolute)=30"} {
		if !strings.Contains(dialplan, route) {
			t.Errorf("missing diagnostic route/application: %s", route)
		}
	}
	for _, number := range []string{"2600", "2602"} {
		if strings.Count(dialplan, "exten => "+number+",1,Goto(gsm-diagnostics,"+number+",1)") != 2 {
			t.Errorf("%s must have exact routes in phones and default", number)
		}
		for _, path := range []string{"docs/API.md", "docs/api/openapi.yaml", "postman/gsm-system.postman_collection.json"} {
			if !strings.Contains(read(path), number) {
				t.Errorf("%s missing from %s", number, path)
			}
		}
	}
}
