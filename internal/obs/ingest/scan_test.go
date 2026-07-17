package ingest

import (
	"bytes"
	"strings"
	"testing"
)

func TestScanLinesBoundsOversizedAndStaysByteAccurate(t *testing.T) {
	big := strings.Repeat("x", maxLineBytes+100)
	data := "a\n" + big + "\nb\n"
	var got []string
	st := &ScanState{}
	off, oversized, err := ScanLines(bytes.NewReader([]byte(data)), 0, func(l []byte) error {
		got = append(got, string(l))
		return nil
	})
	_ = st
	if err != nil {
		t.Fatal(err)
	}
	if off != int64(len(data)) {
		t.Errorf("offset = %d, want %d (must count oversized bytes)", off, len(data))
	}
	if oversized != 1 {
		t.Errorf("oversized = %d, want 1", oversized)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("committed lines = %v, want [a b] (big line skipped)", got)
	}
}

func TestScanLinesTrailingPartialNotCommitted(t *testing.T) {
	off, _, err := ScanLines(bytes.NewReader([]byte("done\npartial-no-newline")), 0, func(l []byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if off != int64(len("done\n")) {
		t.Errorf("offset = %d, want %d (partial line uncommitted)", off, len("done\n"))
	}
}
