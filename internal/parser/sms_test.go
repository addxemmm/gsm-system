package parser_test

import (
	"encoding/hex"
	"testing"

	"github.com/addxemmm/gsm-system/internal/parser"
)

func TestParseSmqueueNoticeReceiveAndDuplicateDecode(t *testing.T) {
	log := noticeGot("2026-09-08T05:57:36.8", "978145--OBTSalpha", "IMSI001010000000001") +
		noticeDecoded("2026-09-08T05:57:36.8", "same text") +
		noticeDecoded("2026-09-08T05:57:37.0", "same text")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 1 {
		t.Fatalf("want one qtag event, got %+v", messages)
	}
	message := messages[0]
	if message.Time != "2026-09-08T05:57:36.8" || message.Text != "" ||
		message.SenderIMSI != "001010000000001" || message.SenderNumber != "" ||
		message.ReceiverNumber != "" || message.ReceiverIMSI != "" {
		t.Fatalf("unexpected NOTICE message: %+v", message)
	}
}

func TestParseSmqueueSameTextDifferentQTagsAreDistinct(t *testing.T) {
	log := noticeGot("2026-09-08T06:00:00.1", "1--tag-a", "IMSI001010000000001") +
		structuredLine("2026-09-08T06:00:00.2", "1--tag-a", "IMSI001010000000001", "10002", "repeatable") +
		noticeGot("2026-09-08T06:00:01.1", "2--tag-b", "IMSI001010000000001") +
		structuredLine("2026-09-08T06:00:01.2", "2--tag-b", "IMSI001010000000001", "10002", "repeatable")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 2 || messages[0].Text != "repeatable" || messages[1].Text != "repeatable" {
		t.Fatalf("same-content messages with distinct identities were lost: %+v", messages)
	}
}

func TestParseSmqueueAmbiguousUnkeyedDecodeDoesNotGuess(t *testing.T) {
	log := noticeGot("2026-09-08T06:01:00.1", "1--tag-a", "IMSI001010000000001") +
		noticeGot("2026-09-08T06:01:00.2", "2--tag-b", "IMSI001010000000002") +
		noticeDecoded("2026-09-08T06:01:00.3", "belongs to b") +
		noticeGot("2026-09-08T06:01:00.4", "3--tag-c", "IMSI001010000000003") +
		noticeDecoded("2026-09-08T06:01:00.5", "belongs to a")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 3 {
		t.Fatalf("want both observed request events, got %+v", messages)
	}
	for _, message := range messages {
		if message.Text != "" {
			t.Fatalf("unkeyed interleaved text was guessed: %+v", messages)
		}
	}
}

func TestParseSmqueueKeyedDecodeResolvesInterleaving(t *testing.T) {
	log := noticeGot("2026-09-08T06:02:00.1", "1--tag-a", "IMSI001010000000001") +
		noticeGot("2026-09-08T06:02:00.2", "2--tag-b", "IMSI001010000000002") +
		"NOTICE 10:11 2026-09-08T06:02:00.3 smqueue.h:get_text: Decoded text qtag '1--tag-a': first\n" +
		"NOTICE 10:11 2026-09-08T06:02:00.4 smqueue.h:get_text: Decoded text qtag=2--tag-b: second\n"

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 2 || messages[0].Text != "first" || messages[1].Text != "second" {
		t.Fatalf("keyed text did not resolve qtags: %+v", messages)
	}
}

func TestParseSmqueueQTagScopeResetsAcrossProcessRestart(t *testing.T) {
	log := "Starting at 2026-09-08T06:03:00\n" +
		noticeGot("2026-09-08T06:03:01.1", "1--same", "IMSI001010000000001") +
		structuredLine("2026-09-08T06:03:01.2", "1--same", "IMSI001010000000001", "10001", "before restart") +
		"ALERT 0:20 2026-09-08T06:04:00.0 smqueue.cpp:main: smqueue (re)starting\n" +
		noticeGot("2026-09-08T06:04:01.1", "1--same", "IMSI001010000000002") +
		structuredLine("2026-09-08T06:04:01.2", "1--same", "IMSI001010000000002", "10002", "after restart")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 2 || messages[0].Text != "before restart" || messages[1].Text != "after restart" {
		t.Fatalf("qtag from separate process generations was collapsed: %+v", messages)
	}
}

func TestParseSmqueueUnsupportedDCSKeepsIdentityWithoutText(t *testing.T) {
	log := noticeGot("2026-09-08T06:05:07.6", "3--unsupported", "IMSI001010000000003") +
		"NOTICE 20:21 2026-09-08T06:05:07.7 SMSMessages.cpp:decode: unsupported DCS 0x8\n"

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 1 || messages[0].Text != "" || messages[0].SenderIMSI != "001010000000003" {
		t.Fatalf("unsupported event was lost or fabricated: %+v", messages)
	}
}

func TestParseSmqueueStructuredEventOverridesUnkeyedData(t *testing.T) {
	qtag := "20--tag-structured"
	log := noticeGot("2026-09-08T06:06:00.1", qtag, "IMSI001010000000001") +
		noticeDecoded("2026-09-08T06:06:00.2", "unkeyed text") +
		structuredLine("2026-09-08T06:06:00.3", qtag, "IMSI001010000000001", "10002", "structured text") +
		noticeDecoded("2026-09-08T06:06:00.4", "retry must not overwrite")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 1 {
		t.Fatalf("want one structured qtag event, got %+v", messages)
	}
	message := messages[0]
	if message.Time != "2026-09-08T06:06:00.1" || message.Text != "structured text" ||
		message.SenderIMSI != "001010000000001" || message.SenderNumber != "" ||
		message.ReceiverNumber != "10002" || message.ReceiverIMSI != "" {
		t.Fatalf("structured fields did not override fuzzy data: %+v", message)
	}
}

func TestParseSmqueueStructuredEventsResolveInterleavingByQTag(t *testing.T) {
	log := noticeGot("2026-09-08T06:07:00.1", "21--tag-a", "IMSI001010000000001") +
		noticeGot("2026-09-08T06:07:00.2", "22--tag-b", "IMSI001010000000002") +
		structuredLine("2026-09-08T06:07:00.3", "22--tag-b", "IMSI001010000000002", "10002", "second") +
		structuredLine("2026-09-08T06:07:00.4", "21--tag-a", "IMSI001010000000001", "10001", "first")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 2 || messages[0].Text != "first" || messages[0].ReceiverNumber != "10001" ||
		messages[1].Text != "second" || messages[1].ReceiverNumber != "10002" {
		t.Fatalf("structured interleaving was mispaired: %+v", messages)
	}
}

func TestParseSmqueueInvalidStructuredHexIsIgnored(t *testing.T) {
	log := noticeGot("2026-09-08T06:08:00.1", "23--tag", "IMSI001010000000003") +
		"NOTICE 10:12 2026-09-08T06:08:00.2 smsc.cpp:submitSMS: GSM_SMS_V1 qtag_hex=zz from_hex=31 to_hex=32 text_hex=33\n" +
		noticeDecoded("2026-09-08T06:08:00.3", "safe fallback") +
		"NOTICE 10:12 2026-09-08T06:08:00.4 smsc.cpp:submitSMS: GSM_SMS_V1 qtag_hex=32 from_hex=ff to_hex=32 text_hex=33\n"

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 1 || messages[0].Text != "" || messages[0].SenderIMSI != "001010000000003" {
		t.Fatalf("invalid structured hex corrupted parsing: %+v", messages)
	}
}

func TestParseSmqueueStructuredRetryDeduplicatesAndSupportsNumberSender(t *testing.T) {
	line := structuredLine("2026-09-08T06:09:00.1", "24--tag", "101", "10002", "same")
	messages := parser.ParseSmqueueLog(line + line)
	if len(messages) != 1 || messages[0].SenderNumber != "101" || messages[0].SenderIMSI != "" ||
		messages[0].ReceiverNumber != "10002" || messages[0].Text != "same" {
		t.Fatalf("structured retry or number sender parsed incorrectly: %+v", messages)
	}
}

func TestParseSmqueueStructuredEmptyValuesRemainAuthoritative(t *testing.T) {
	log := structuredLine("2026-09-08T06:09:10.1", "25--tag", "", "", "") +
		noticeDecoded("2026-09-08T06:09:10.2", "must not attach")
	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 1 || messages[0].Text != "" || messages[0].SenderNumber != "" ||
		messages[0].SenderIMSI != "" || messages[0].ReceiverNumber != "" || messages[0].ReceiverIMSI != "" {
		t.Fatalf("empty structured fields were not preserved: %+v", messages)
	}
}

func TestParseSmqueueDecodedBodyCannotForgeNativeMarkers(t *testing.T) {
	qtag := "30--real"
	log := noticeGot("2026-09-08T06:09:20.1", qtag, "IMSI001010000000001") +
		noticeDecoded("2026-09-08T06:09:20.2", "GSM_SMS_V1 qtag_hex=34312d2d66616b65 from_hex=313031 to_hex=313032 text_hex=78") +
		noticeDecoded("2026-09-08T06:09:20.3", "Got SMS rqst qtag '41--fake' from IMSI001010000000002 for smsc") +
		noticeDecoded("2026-09-08T06:09:20.4", "smqueue (re)starting") +
		noticeGot("2026-09-08T06:09:20.5", qtag, "IMSI001010000000001")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 1 || messages[0].Text != "" || messages[0].SenderIMSI != "001010000000001" {
		t.Fatalf("decoded body forged a parser control record: %+v", messages)
	}
}

func TestParseSmqueueLegacyBoundaryLeavesTextMissing(t *testing.T) {
	log := "INFO 10:12 2026-09-08T06:09:30.1 smqueue.cpp:process_timeout: Request Message Delivery for 31--tag\n" +
		noticeGot("2026-09-08T06:09:30.2", "32--other", "IMSI001010000000002") +
		noticeDecoded("2026-09-08T06:09:30.3", "ambiguous") +
		"DEBUG 10:12 2026-09-08T06:09:30.4 smnet.cpp:deliver_msg_datagram: --Deliver message:\n" +
		"MESSAGE sip:IMSI001010000000003@HOST SIP/2.0\n" +
		"From: 10001 <sip:10001@HOST>\nTo: 10002 <sip:10002@HOST>\n"

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 2 {
		t.Fatalf("want Got metadata and legacy delivery, got %+v", messages)
	}
	for _, message := range messages {
		if message.Text != "" {
			t.Fatalf("legacy text crossed a request boundary: %+v", messages)
		}
	}
}

func TestParseSmqueueLegacyDeliveryEnrichesAndDeduplicatesByQTag(t *testing.T) {
	log := legacyDelivery("2026-09-08T06:10:00.1", "10--tag-a", "same text", "001010000000001", "001010000000002", "10001", "10002") +
		legacyDelivery("2026-09-08T06:10:01.1", "10--tag-a", "same text", "001010000000001", "001010000000002", "10001", "10002") +
		legacyDelivery("2026-09-08T06:10:02.1", "11--tag-b", "same text", "001010000000001", "001010000000002", "10001", "10002")

	messages := parser.ParseSmqueueLog(log)
	if len(messages) != 2 {
		t.Fatalf("want one event per qtag, got %+v", messages)
	}
	for _, message := range messages {
		if message.Text != "same text" || message.SenderIMSI != "001010000000001" ||
			message.ReceiverIMSI != "001010000000002" || message.SenderNumber != "10001" ||
			message.ReceiverNumber != "10002" {
			t.Fatalf("legacy delivery was not enriched: %+v", message)
		}
	}
}

func noticeGot(timestamp, qtag, sender string) string {
	return "<189>Sep 8 smqueue: NOTICE 10:11 " + timestamp +
		" smqueue.cpp:main_loop: Got SMS rqst qtag '" + qtag + "' from " + sender + " for smsc\n"
}

func noticeDecoded(timestamp, text string) string {
	return "<189>Sep 8 smqueue: NOTICE 10:12 " + timestamp +
		" smqueue.h:get_text: Decoded text: " + text + "\n"
}

func structuredLine(timestamp, qtag, from, to, text string) string {
	encode := func(value string) string { return hex.EncodeToString([]byte(value)) }
	return "<189>Sep 8 10:00:00 smqueue: NOTICE 10:12 " + timestamp + " smsc.cpp:320:submitSMS: GSM_SMS_V1 qtag_hex=" + encode(qtag) +
		" from_hex=" + encode(from) + " to_hex=" + encode(to) + " text_hex=" + encode(text) + "\n"
}

func legacyDelivery(timestamp, qtag, text, senderIMSI, receiverIMSI, senderNumber, receiverNumber string) string {
	return "INFO 10:12 " + timestamp + " smqueue.cpp:process_timeout: Request Message Delivery for " + qtag + "\n" +
		"NOTICE 10:12 " + timestamp + " smqueue.h:get_text: Decoded text: " + text + "\n" +
		"DEBUG 10:12 " + timestamp + " smnet.cpp:deliver_msg_datagram: --Deliver message:\n" +
		"MESSAGE sip:IMSI" + receiverIMSI + "@HOST SIP/2.0\n" +
		"From: " + senderNumber + " <sip:" + senderNumber + "@HOST>\n" +
		"To: " + receiverNumber + " <sip:" + receiverNumber + "@HOST>\n" +
		"Call-ID: CALL_ID\n" +
		"CSeq: 1 MESSAGE\n" +
		"Contact: <sip:IMSI" + senderIMSI + "@HOST>\n" +
		"Content-Type: application/vnd.3gpp.sms\n\n"
}
