package env

import (
	"os"
	"testing"
)

func TestGetStr(t *testing.T) {
	const key = "MELISSA_TEST_STR"
	t.Cleanup(func() { _ = os.Unsetenv(key) })

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	if got := GetStr(key, "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}

	if err := os.Setenv(key, "value"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if got := GetStr(key, "fallback"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}

func TestGetBool(t *testing.T) {
	const key = "MELISSA_TEST_BOOL"
	t.Cleanup(func() { _ = os.Unsetenv(key) })

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	if got := GetBool(key, true); !got {
		t.Fatalf("expected default true when env is unset")
	}

	if err := os.Setenv(key, "false"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if got := GetBool(key, true); got {
		t.Fatalf("expected parsed false")
	}

	if err := os.Setenv(key, "not-a-bool"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if got := GetBool(key, false); got {
		t.Fatalf("expected default false for invalid boolean")
	}
}
