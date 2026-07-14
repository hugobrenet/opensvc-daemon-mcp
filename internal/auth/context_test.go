package auth

import (
	"context"
	"net/http"
	"testing"
)

func TestApplyBearerFromContext(t *testing.T) {
	ctx := WithBearerToken(context.Background(), "delegated-token")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	if err := ApplyBearerFromContext(request); err != nil {
		t.Fatalf("apply delegated token: %v", err)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer delegated-token" {
		t.Errorf("got Authorization header %q, want delegated Bearer token", got)
	}
}

func TestApplyBearerFromContextRejectsMissingToken(t *testing.T) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	if err := ApplyBearerFromContext(request); err == nil {
		t.Fatal("ApplyBearerFromContext succeeded, want an error")
	}
}
