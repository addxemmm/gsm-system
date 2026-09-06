package api

import "testing"

func TestAnyCellServiceRunningIncludesDependencies(t *testing.T) {
	for _, process := range []string{"OpenBTS", "transceiver", "sipauthserve", "smqueue", "asterisk"} {
		t.Run(process, func(t *testing.T) {
			if !anyCellServiceRunning(func(name string) bool { return name == process }) {
				t.Fatalf("%s must prevent a successful stopped response", process)
			}
		})
	}
	if anyCellServiceRunning(func(string) bool { return false }) {
		t.Fatal("no processes should report stopped")
	}
}
