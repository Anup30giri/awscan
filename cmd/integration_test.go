package cmd

import (
	"os"
	"testing"
)

func TestCommandIntegrationScaffold(t *testing.T) {
	if os.Getenv("AWSCAN_INTEGRATION") == "" {
		t.Skip("set AWSCAN_INTEGRATION=1 to enable integration tests")
	}
}
