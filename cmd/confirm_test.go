package cmd

import (
	"strings"
	"testing"
)

func TestConfirmActionYes(t *testing.T) {
	t.Parallel()

	ok, err := confirmAction(strings.NewReader("yes\n"), &strings.Builder{}, "Proceed?")
	if err != nil {
		t.Fatalf("confirmAction() error = %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation to be true")
	}
}
