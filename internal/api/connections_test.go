package api

import (
	"encoding/json"
	"testing"

	"github.com/addxemmm/gsm-system/internal/parser"
)

func TestConnectionItemsExposeRejectionWithoutInventingNumberOrIP(t *testing.T) {
	auth, reject := 0, 4
	items := connectionItems([]parser.UE{{
		IMSI: "001010000000001", IMEI: "000000000000001", Auth: &auth, RejectCode: &reject,
	}})
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0]["auth"] != float64(0) || decoded[0]["reject_code"] != float64(4) {
		t.Fatalf("raw registration diagnostics lost: %s", raw)
	}
	if decoded[0]["number"] != nil || decoded[0]["ip"] != nil {
		t.Fatalf("rejection must not invent service identity or data IP: %s", raw)
	}
	if _, exists := decoded[0]["connected"]; exists {
		t.Fatalf("observed TMSI record is not a live connection claim: %s", raw)
	}
}

func TestConnectionItemsKeepUnknownDiagnosticValuesNull(t *testing.T) {
	items := connectionItems([]parser.UE{{IMSI: "001010000000002"}})
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"auth", "reject_code", "imei", "number", "ip"} {
		value, exists := decoded[0][key]
		if !exists || value != nil {
			t.Errorf("%s must remain explicit null when unknown: %s", key, raw)
		}
	}
	if empty := connectionItems(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("empty connection list must be [], got %#v", empty)
	}
}
