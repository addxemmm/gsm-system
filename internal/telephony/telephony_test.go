package telephony

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseConciseAsterisk18Fields(t *testing.T) {
	row := "SIP/IMSI001-0001!phones!10002!1!Up!Dial!SIP/IMSI002!10001!!peer!3!12!bridge-1!169.1\n" +
		"1 active channel\n1 active call\n"
	calls, err := ParseConcise(row)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].DurationSeconds != 12 || calls[0].BridgeID == nil ||
		*calls[0].BridgeID != "bridge-1" || calls[0].UniqueID == nil || *calls[0].UniqueID != "169.1" {
		t.Fatalf("unexpected call: %+v", calls)
	}
}

func TestParseConciseEmptyAndMalformedOutput(t *testing.T) {
	for _, output := range []string{"", "0 active channels\n0 active calls\n"} {
		calls, err := ParseConcise(output)
		if err != nil || len(calls) != 0 {
			t.Fatalf("empty output %q calls=%v err=%v", output, calls, err)
		}
	}
	for _, output := range []string{
		"Unable to connect to remote asterisk\n",
		"a!b!c!d!e!f!g!h!i!j!12!bridge!uid\n",
		"a!b!c!d!e!f!g!h!i!j!k!bad!bridge!uid\n",
	} {
		if _, err := ParseConcise(output); err == nil {
			t.Fatalf("accepted malformed output %q", output)
		}
	}
}

func TestActiveParsesStdoutAndIgnoresAsteriskStderrWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	script := writeExecutable(t, `#!/bin/sh
printf '%s\n' 'SIP/IMSI001-0001!phones!10002!1!Up!Dial!SIP/IMSI002!10001!!peer!3!12!bridge-1!169.1'
printf '%s\n' 'No ethernet interface found for seeding global EID' >&2
`)
	calls, err := Active(context.Background(), script)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].UniqueID == nil || *calls[0].UniqueID != "169.1" {
		t.Fatalf("calls=%+v", calls)
	}
}

func TestActiveRejectsExitZeroCommandErrorOnStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	script := writeExecutable(t, "#!/bin/sh\nprintf '%s\\n' 'No such command core show channels concise'\n")
	if _, err := Active(context.Background(), script); err == nil {
		t.Fatal("exit-zero command error was accepted as an empty channel list")
	}
}

func TestActiveRejectsNonzeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	script := writeExecutable(t, "#!/bin/sh\nprintf '%s\\n' 'native failure' >&2\nexit 1\n")
	if _, err := Active(context.Background(), script); err == nil {
		t.Fatal("nonzero asterisk CLI exit was accepted")
	}
}

func TestHistoryParses18ColumnsAndNormalizesUTC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Master.csv")
	writeCDR(t, path, [][]string{{
		"acct", "10001", "10002", "phones", `"Caller" <10001>`, "SIP/a", "SIP/b", "Dial", "SIP/b",
		"2026-09-08 01:02:03", "", "2026-09-08 01:02:15", "12", "0", "NO ANSWER", "DOCUMENTATION", "169.1", "note\nline",
	}})
	calls, window, err := History(context.Background(), path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if window.Truncated || len(calls) != 1 {
		t.Fatalf("window=%+v calls=%v", window, calls)
	}
	call := calls[0]
	if call.StartedAt != "2026-09-08T01:02:03Z" || call.EndedAt != "2026-09-08T01:02:15Z" ||
		call.AnsweredAt != nil || call.UniqueID == nil || *call.UniqueID != "169.1" ||
		call.UserField == nil || *call.UserField != "note\nline" {
		t.Fatalf("unexpected call: %+v", call)
	}
}

func TestHistoryKeepsOnlyCompleteRecordsInTailWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Master.csv")
	rows := [][]string{
		cdrRow("1", strings.Repeat("quoted\n", 40)),
		cdrRow("2", "second"),
		cdrRow("3", "third"),
	}
	writeCDR(t, path, rows)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	calls, window, err := History(context.Background(), path, info.Size()/2)
	if err != nil {
		t.Fatal(err)
	}
	if !window.Truncated || len(calls) == 0 {
		t.Fatalf("window=%+v calls=%v", window, calls)
	}
	for _, call := range calls {
		if call.UniqueID != nil && *call.UniqueID == "1" {
			t.Fatal("record crossing the tail threshold was retained")
		}
	}
}

func TestHistoryRejectsWrongWidthAndInvalidNumbers(t *testing.T) {
	for name, row := range map[string][]string{
		"width":  {"only", "two"},
		"number": append(cdrRow("1", "")[:12], append([]string{"bad"}, cdrRow("1", "")[13:]...)...),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Master.csv")
			writeCDR(t, path, [][]string{row})
			if _, _, err := History(context.Background(), path, 1<<20); err == nil {
				t.Fatal("malformed CDR accepted")
			}
		})
	}
}

func TestHistoryHonorsCanceledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Master.csv")
	writeCDR(t, path, [][]string{cdrRow("1", "")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := History(ctx, path, 1<<20); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestHistoryRejectsIncompleteEOFInTruncatedWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Master.csv")
	if err := os.WriteFile(path, []byte(strings.Repeat("old\n", 40)+`"partial`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := History(context.Background(), path, 64); err == nil ||
		!strings.Contains(err.Error(), "incomplete record") {
		t.Fatalf("error=%v", err)
	}
}

func TestHistoryAllowsCompleteNonTruncatedRecordWithoutFinalNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Master.csv")
	writeCDR(t, path, [][]string{cdrRow("1", "")})
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.TrimSuffix(b, []byte("\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	calls, _, err := History(context.Background(), path, 1<<20)
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%v error=%v", calls, err)
	}
}

func cdrRow(uniqueID, userField string) []string {
	return []string{"", "10001", "10002", "phones", "caller", "SIP/a", "SIP/b", "Dial", "SIP/b",
		"2026-09-08 01:02:03", "2026-09-08 01:02:04", "2026-09-08 01:02:15", "12", "11",
		"ANSWERED", "DOCUMENTATION", uniqueID, userField}
}

func writeCDR(t *testing.T, path string, rows [][]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := csv.NewWriter(file)
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "asterisk-fixture")
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConciseKnownEIDNoticeOnly(t *testing.T) {
	valid := "Local/test@fixture-1;1!fixture!test!1!Up!Wait!1!10001!!!3!2!bridge-uuid!123.45"
	calls, err := ParseConcise("No ethernet interface found for seeding global EID. You will have to set it manually.\n" + valid)
	if err != nil || len(calls) != 1 || calls[0].UniqueID == nil || *calls[0].UniqueID != "123.45" {
		t.Fatalf("known native notice: calls=%+v err=%v", calls, err)
	}
	if _, err := ParseConcise("No ethernet interface found for another reason.\n" + valid); err == nil {
		t.Fatal("unexpected native output was silently swallowed")
	}
}
