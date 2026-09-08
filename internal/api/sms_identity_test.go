package api

import (
	"reflect"
	"testing"

	"github.com/addxemmm/gsm-system/internal/parser"
	"github.com/addxemmm/gsm-system/internal/subscriber"
)

func stringPointer(value string) *string { return &value }

func TestSMSIdentityResolutionUsesOnlyUniqueConsistentCurrentBindings(t *testing.T) {
	bindings := newSMSBindingIndex([]subscriber.Subscriber{
		{IMSI: "001010000000011", Number: stringPointer("70000011"), Consistent: true},
		{IMSI: "001010000000012", Number: stringPointer("70000012"), Consistent: true},
	})
	message := parser.SMS{SenderIMSI: "001010000000011", ReceiverNumber: "70000012"}
	resolution := smsIdentityResolution(&message, bindings)

	if message.SenderNumber != "70000011" || message.ReceiverIMSI != "001010000000012" {
		t.Fatalf("unique current bindings were not completed: %+v", message)
	}
	want := map[string]string{
		"sender_number": identityCurrentBinding, "sender_imsi": identityLogObservation,
		"receiver_number": identityLogObservation, "receiver_imsi": identityCurrentBinding,
	}
	if !reflect.DeepEqual(resolution, want) {
		t.Fatalf("resolution=%v, want %v", resolution, want)
	}
}

func TestSMSIdentityResolutionNeverOverridesLogObservations(t *testing.T) {
	bindings := newSMSBindingIndex([]subscriber.Subscriber{
		{IMSI: "001010000000021", Number: stringPointer("70000021"), Consistent: true},
	})
	message := parser.SMS{
		SenderIMSI: "001010000000021", SenderNumber: "79999999",
		ReceiverIMSI: "001010000000099", ReceiverNumber: "70000021",
	}
	resolution := smsIdentityResolution(&message, bindings)

	if message.SenderNumber != "79999999" || message.ReceiverIMSI != "001010000000099" {
		t.Fatalf("log observation was overwritten: %+v", message)
	}
	for field, source := range resolution {
		if source != identityLogObservation {
			t.Fatalf("%s source=%q, want log observation", field, source)
		}
	}
}

func TestSMSIdentityResolutionRejectsAmbiguousAndInconsistentMappings(t *testing.T) {
	bindings := newSMSBindingIndex([]subscriber.Subscriber{
		{IMSI: "001010000000031", Number: stringPointer("70000031"), Consistent: true},
		{IMSI: "001010000000032", Number: stringPointer("70000031"), Consistent: true},
		{IMSI: "001010000000033", Number: stringPointer("70000033"), Consistent: false},
		{IMSI: "001010000000034", Number: stringPointer("70000034"), Consistent: true},
		{IMSI: "001010000000034", Number: stringPointer("70000035"), Consistent: true},
	})
	for _, message := range []parser.SMS{
		{SenderIMSI: "001010000000031"},
		{SenderNumber: "70000031"},
		{SenderIMSI: "001010000000033"},
		{SenderIMSI: "001010000000034"},
	} {
		resolution := smsIdentityResolution(&message, bindings)
		if message.SenderNumber != "" && resolution["sender_number"] != identityLogObservation {
			t.Fatalf("unexpected ambiguous completion: %+v %v", message, resolution)
		}
		if message.SenderIMSI != "" && resolution["sender_imsi"] != identityLogObservation {
			t.Fatalf("unexpected ambiguous completion: %+v %v", message, resolution)
		}
		if message.SenderNumber != "" && message.SenderIMSI != "" {
			t.Fatalf("ambiguous mapping completed both identities: %+v", message)
		}
	}
}

func TestSMSIdentityResolutionLeavesServiceCodesUnknown(t *testing.T) {
	for _, code := range []string{"101", "411", "111", "112", "911"} {
		bindings := newSMSBindingIndex([]subscriber.Subscriber{
			{IMSI: "001010000000041", Number: stringPointer(code), Consistent: true},
		})
		message := parser.SMS{SenderNumber: code}
		resolution := smsIdentityResolution(&message, bindings)
		if message.SenderIMSI != "" || resolution["sender_imsi"] != identityUnknown ||
			resolution["sender_number"] != identityLogObservation {
			t.Fatalf("service code %s acquired a subscriber identity: %+v %v", code, message, resolution)
		}
	}
}
