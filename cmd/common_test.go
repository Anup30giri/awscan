package cmd

import "testing"

func TestServiceTargetOptions(t *testing.T) {
	t.Parallel()

	options := serviceTargetOptions()
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2", len(options))
	}
	if options[0].Value != "ecs" || options[1].Value != "ec2" {
		t.Fatalf("unexpected options: %+v", options)
	}
}
