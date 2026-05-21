package diagnostics

import "testing"

func TestReportString(t *testing.T) {
	t.Parallel()

	report := &Report{}
	report.Add("AWS CLI", StatusPass, "installed")
	if got := report.String(); got == "" {
		t.Fatal("Report.String() returned empty output")
	}
}

func TestClassifyOperationMessage(t *testing.T) {
	t.Parallel()

	got := classifyOperationMessage("ECS ListClusters", "ecs:ListClusters", assertErr("AccessDeniedException"))
	if got == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestClassifyOperationMessageEndpoint(t *testing.T) {
	t.Parallel()

	got := classifyOperationMessage("ECS ListClusters", "ecs:ListClusters", assertErr("endpoint url error"))
	if got == "" {
		t.Fatal("expected non-empty endpoint message")
	}
}

type stringErr string

func (e stringErr) Error() string { return string(e) }

func assertErr(text string) error { return stringErr(text) }
