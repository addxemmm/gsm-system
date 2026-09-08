package parser_test

import (
	"os"
	"testing"

	"github.com/addxemmm/gsm-system/internal/parser"
)

func TestParseSmqueueSample(t *testing.T) {
	b, err := os.ReadFile("../../gsmsystem/log/smqueue.log")
	if err != nil {
		t.Skipf("sample log absent: %v", err)
	}
	sms := parser.ParseSmqueueLog(string(b))
	if len(sms) == 0 {
		t.Fatal("want >=1 SMS from sample log")
	}
	found := false
	for _, m := range sms {
		if m.Text == "abcdefg" {
			found = true
			if m.SenderIMSI == "" && m.SenderNumber == "" {
				t.Fatalf("decoded SMS missing sender: %+v", m)
			}
		}
	}
	if !found {
		t.Fatalf("sample text abcdefg not found in %d messages", len(sms))
	}
}

func TestParseSGSNAndTMSIs(t *testing.T) {
	sgsn := parser.ParseSGSN("GMM Context: imsi=001010000000001 state=GmmRegisteredNormal IPs=invalid,192.0.2.2,192.0.2.3\n" +
		"GMM Context: imsi=001010000000002 state=GmmDeregistered IPs=none\n" +
		"IMSI=001010000000003 state=legacy IP=not-an-address\n")
	if sgsn["001010000000001"] != "192.0.2.2" || sgsn["001010000000002"] != "" || sgsn["001010000000003"] != "" {
		t.Fatalf("sgsn parse: %v", sgsn)
	}

	header := "IMSI            TMSI IMEI            AUTH CREATED ACCESSED TMSI_ASSIGNED PTMSI_ASSIGNED AUTH_EXPIRY REJECT_CODE ASSOCIATED_URI   ASSERTED_IDENTITY WELCOME_SENT"
	rows := []map[string]string{
		{"IMSI": "001010000000001", "TMSI": "-", "IMEI": "350000000000001", "AUTH": "0", "CREATED": "1m", "ACCESSED": "1m", "TMSI_ASSIGNED": "0", "PTMSI_ASSIGNED": "0", "AUTH_EXPIRY": "-", "REJECT_CODE": "4", "WELCOME_SENT": "1"},
		{"IMSI": "001010000000002", "TMSI": "-", "IMEI": "350000000000002", "AUTH": "0", "CREATED": "1m", "ACCESSED": "1m", "TMSI_ASSIGNED": "0", "PTMSI_ASSIGNED": "0", "AUTH_EXPIRY": "-", "REJECT_CODE": "4", "WELCOME_SENT": "1"},
		{"IMSI": "001010000000003", "TMSI": "-", "IMEI": "350000000000003", "AUTH": "1", "CREATED": "1m", "ACCESSED": "1m", "TMSI_ASSIGNED": "0", "PTMSI_ASSIGNED": "0", "AUTH_EXPIRY": "-", "REJECT_CODE": "0", "ASSOCIATED_URI": "<tel:10003>", "WELCOME_SENT": "0"},
	}
	out := header + "\n"
	for _, row := range rows {
		out += fixedWidthRow(header, row) + "\n"
	}
	ues := parser.ParseTMSIs(out, sgsn)
	if len(ues) != 3 {
		t.Fatalf("want 3 UEs, got %v", ues)
	}
	if ues[0].IP != "192.0.2.2" {
		t.Fatalf("UE IP join failed: %+v", ues[0])
	}
	if ues[0].Number != "" || ues[1].Number != "" {
		t.Fatalf("WELCOME_SENT must not become a number: %+v", ues)
	}
	if ues[2].Number != "10003" {
		t.Fatalf("telephone URI parse failed: %+v", ues[2])
	}
	if ues[0].Auth == nil || *ues[0].Auth != 0 || ues[0].RejectCode == nil || *ues[0].RejectCode != 4 {
		t.Fatalf("raw diagnostics parse failed: %+v", ues[0])
	}
}

func TestParseTMSIsInvalidDiagnosticsAreUnknown(t *testing.T) {
	header := "IMSI            TMSI IMEI            AUTH CREATED ACCESSED TMSI_ASSIGNED PTMSI_ASSIGNED AUTH_EXPIRY REJECT_CODE ASSOCIATED_URI   ASSERTED_IDENTITY WELCOME_SENT"
	row := fixedWidthRow(header, map[string]string{
		"IMSI": "001010000000004", "TMSI": "-", "IMEI": "350000000000004",
		"AUTH": "?", "REJECT_CODE": "bad", "WELCOME_SENT": "1",
	})
	ues := parser.ParseTMSIs(header+"\n"+row+"\n", nil)
	if len(ues) != 1 || ues[0].Auth != nil || ues[0].RejectCode != nil {
		t.Fatalf("invalid diagnostics must remain unknown: %+v", ues)
	}
}

func fixedWidthRow(header string, values map[string]string) string {
	names := []string{"IMSI", "TMSI", "IMEI", "AUTH", "CREATED", "ACCESSED", "TMSI_ASSIGNED", "PTMSI_ASSIGNED", "AUTH_EXPIRY", "REJECT_CODE", "ASSOCIATED_URI", "ASSERTED_IDENTITY", "WELCOME_SENT"}
	row := make([]byte, len(header))
	for i := range row {
		row[i] = ' '
	}
	searchFrom := 0
	for _, name := range names {
		start := indexFrom(header, name, searchFrom)
		copy(row[start:], values[name])
		searchFrom = start + len(name)
	}
	return string(row)
}

func indexFrom(s, substr string, start int) int {
	for i := start; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
