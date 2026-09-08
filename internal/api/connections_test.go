package api

import (
	"encoding/json"
	"testing"

	"github.com/addxemmm/gsm-system/internal/parser"
	"github.com/addxemmm/gsm-system/internal/subscriber"
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

func TestConnectionNumbersUseRegistryAndClearStaleCache(t *testing.T) {
	items := connectionItems([]parser.UE{
		{IMSI: "001010000000001"},
		{IMSI: "001010000000002", Number: "10002"},
		{IMSI: "001010000000003", Number: "10003"},
		{IMSI: "001010000000004", Number: "10004"},
	})
	number := "10001"
	applySubscriberNumbers(items, []subscriber.Subscriber{
		{IMSI: "001010000000001", Number: &number, Consistent: true},
		{IMSI: "001010000000002", Consistent: true},
		{IMSI: "001010000000003", Number: &number, Consistent: false},
	})
	if items[0]["number"] != "10001" || items[0]["number_source"] != "subscriber_registry" {
		t.Fatal("native SMS binding not projected")
	}
	if items[1]["number"] != nil || items[1]["number_source"] != "subscriber_registry" {
		t.Fatal("stale TMSI number survived an authoritative unbind")
	}
	if items[2]["number"] != nil || items[2]["number_source"] != "inconsistent_registry" {
		t.Fatal("inconsistent binding exposed as a valid number")
	}
	if items[3]["number"] != "10004" || items[3]["number_source"] != "openbts_tmsi" {
		t.Fatal("unregistered TMSI fallback lost")
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
