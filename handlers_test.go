package main

import (
	"strings"
	"testing"
)

func TestValidateChirp(t *testing.T) {
	tests := map[string]struct {
		input   string
		wantErr string // empty string means no error expected
	}{
		"valid":    {input: "hello world", wantErr: ""},
		"too long": {input: "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateChirp(tc.input)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
