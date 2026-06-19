package dialog

import (
	"context"
	"runtime"
	"testing"

	output "github.com/shhac/lib-agent-output"
)

// forceAvailable clears the env vars Available() inspects so the prompt path
// runs regardless of the test host (e.g. CI over SSH, or headless Linux).
func forceAvailable(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
}

func withBackend(t *testing.T, fn func(ctx context.Context, title string, f Field) (string, error)) {
	prev := promptOne
	promptOne = fn
	t.Cleanup(func() { promptOne = prev })
}

func TestPromptMapsFieldsByID(t *testing.T) {
	forceAvailable(t)
	withBackend(t, func(_ context.Context, _ string, f Field) (string, error) {
		return "value-" + f.ID, nil
	})

	res, err := Prompt(context.Background(), Spec{Title: "t", Fields: []Field{
		{ID: "token", Label: "Token", Hidden: true},
		{ID: "cookie", Label: "Cookie"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res["token"] != "value-token" || res["cookie"] != "value-cookie" {
		t.Errorf("results = %v", res)
	}
}

func TestPromptSecretIsHiddenSingleField(t *testing.T) {
	forceAvailable(t)
	withBackend(t, func(_ context.Context, _ string, f Field) (string, error) {
		if !f.Hidden {
			t.Error("PromptSecret field should be hidden")
		}
		return "s3cret", nil
	})

	got, err := PromptSecret(context.Background(), "title", "API token")
	if err != nil || got != "s3cret" {
		t.Errorf("PromptSecret = %q, %v", got, err)
	}
}

func TestAvailableHeadlessLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("headless check is Linux-specific")
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	err := Available()
	if err == nil {
		t.Fatal("headless linux should be unavailable")
	}
	var oe *output.Error
	if !output.As(err, &oe) || oe.FixableBy != output.FixableByHuman {
		t.Errorf("want fixable_by:human structured error, got %v", err)
	}
}
