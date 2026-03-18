package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()

	validToken, err := MakeJWT(userID, "correct-secret", time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT returned error: %v", err)
	}

	expiredToken, err := MakeJWT(userID, "correct-secret", -time.Second)
	if err != nil {
		t.Fatalf("MakeJWT returned error: %v", err)
	}

	tests := map[string]struct {
		token   string
		secret  string
		wantID  uuid.UUID
		wantErr bool
	}{
		"valid token":   {token: validToken, secret: "correct-secret", wantID: userID, wantErr: false},
		"expired token": {token: expiredToken, secret: "correct-secret", wantErr: true},
		"wrong secret":  {token: validToken, secret: "wrong-secret", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotID, err := ValidateJWT(tc.token, tc.secret)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotID != tc.wantID {
				t.Errorf("got userID %v, want %v", gotID, tc.wantID)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	tests := map[string]struct {
		input   http.Header
		want    string
		wantErr bool
	}{
		"valid input": {input: http.Header{"Authorization": []string{"Bearer abc123"}}, want: "abc123", wantErr: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := GetBearerToken(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("Mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
