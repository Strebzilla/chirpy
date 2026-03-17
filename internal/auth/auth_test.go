package auth

import (
	"testing"
	"time"

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
