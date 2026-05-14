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
