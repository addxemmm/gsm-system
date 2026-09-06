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
	sgsn := parser.ParseSGSN("IMSI=001010123456780 foo bar IP=192.168.99.2\n")
	if sgsn["001010123456780"] != "192.168.99.2" {
		t.Fatalf("sgsn parse: %v", sgsn)
	}
	ues := parser.ParseTMSIs("IMSI IMEI ... header\n001010123456780 x 351615087961130 a b c d e f g tel:10000001) extra\n", sgsn)
	if len(ues) != 1 {
		t.Fatalf("want 1 UE, got %v", ues)
	}
	if ues[0].IP != "192.168.99.2" {
		t.Fatalf("UE IP join failed: %+v", ues[0])
	}
}
