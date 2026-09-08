package gsm_test

import (
	"testing"

	"github.com/addxemmm/gsm-system/internal/gsm"
)

func TestValidateOK(t *testing.T) {
	p := gsm.StartParams{ARFCNs: "1", C0: "540", Band: "1800", MCC: "001",
		MNC: "01", LAC: "4420", CI: "41240", ShortName: "test", Network: "eth0"}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if issues := p.ValidateDetailed(); len(issues) != 0 {
		t.Fatalf("want no issues, got %v", issues)
	}
}

func TestValidateBandStrict(t *testing.T) {
	p := gsm.StartParams{ARFCNs: "1", C0: "540", Band: "7", MCC: "001",
		MNC: "01", LAC: "4420", CI: "41240", ShortName: "test", Network: "eth0"}
	if err := p.Validate(); err == nil {
		t.Fatal("LTE band 7 must not validate as GSM band")
	}
	if issues := p.ValidateDetailed(); len(issues) == 0 {
		t.Fatal("detailed must report band issue")
	}
}

func TestValidateIMSI(t *testing.T) {
	if err := gsm.ValidateIMSI("001010123456780"); err != nil {
		t.Fatal(err)
	}
	if err := gsm.ValidateIMSI("abc"); err == nil {
		t.Fatal("bad imsi must fail")
	}
}
