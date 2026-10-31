package main

import (
	"strings"
	"testing"
)

func TestMonFrame(t *testing.T) {
	tests := []struct {
		name      string
		full      bool
		wantClear bool
	}{
		{"tick overwrites in place", false, false},
		{"resize clears", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := monFrame("a\nb\n", tt.full)
			if strings.Contains(got, clearScreen) != tt.wantClear {
				t.Fatalf("clearScreen present = %v, want %v", !tt.wantClear, tt.wantClear)
			}
			if !strings.HasPrefix(got, beginSync) || !strings.HasSuffix(got, eraseDown+endSync) {
				t.Fatalf("frame not wrapped in sync/eraseDown: %q", got)
			}
			if !strings.Contains(got, "a"+eraseLine+"\nb"+eraseLine+"\n") {
				t.Fatalf("lines not erased to EOL: %q", got)
			}
		})
	}
}
