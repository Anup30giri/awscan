package diagnostics

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	internalaws "github.com/anupgiri/awscan/internal/aws"
	internalexec "github.com/anupgiri/awscan/internal/exec"
	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
)

type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
)

type Check struct {
	Name    string
	Status  Status
	Details string
}

type Report struct {
	Checks []Check
}

type Options struct {
	Profile string
	Region  string
	Cluster string
	Service string
	Task    string
	Instance string
}

type Doctor struct {
	Resolver internalaws.Resolver
	Runner   internalexec.Runner
}

func NewDoctor(resolver internalaws.Resolver, runner internalexec.Runner) *Doctor {
	return &Doctor{
		Resolver: resolver,
		Runner:   runner,
	}
}

func (d *Doctor) Run(ctx context.Context, opts Options) (*Report, error) {
	report := &Report{}

	awsVersion, awsErr := binaryVersion(ctx, "aws", "--version")
	report.Add("AWS CLI", statusFromErr(awsErr), firstNonEmpty(awsVersion, errorString(awsErr)))

	smVersion, smErr := binaryVersion(ctx, "session-manager-plugin", "--version")
	report.Add("session-manager-plugin", statusFromErr(smErr), firstNonEmpty(smVersion, errorString(smErr)))

	runtime, err := d.Resolver.Resolve(ctx, internalaws.ResolveOptions{
		Profile: opts.Profile,
		Region:  opts.Region,
	})
	if err != nil {
		report.Add("AWS SDK credential loading", StatusFail, err.Error())
		return report, nil
	}

	report.Add("Effective AWS profile", StatusPass, firstNonEmpty(runtime.Profile, envProfileHint()))
	report.Add("Effective AWS region", StatusPass, runtime.Region)
	report.Add("Profile type", StatusPass, string(runtime.ProfileKind))

	if runtime.ProfileKind == internalaws.ProfileKindLoginSession {
		provider := internalaws.NewExportedCredentialProvider(runtime.Profile)
		_, err := provider.Retrieve(ctx)
		report.Add("login_session export-credentials", statusFromErr(err), okOrError("AWS CLI export-credentials works for this profile", err))
	}

	identity, err := internalaws.GetCallerIdentity(ctx, runtime)
	if err != nil {
		report.Add("STS caller identity", StatusFail, err.Error())
		return report, nil
	}
	report.Add("STS caller identity", StatusPass, fmt.Sprintf("account=%s arn=%s", identity.Account, identity.ARN))

	ecsProvider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, d.Runner)
	clusters, err := ecsProvider.ListClusters(ctx)
	if err != nil {
		report.Add("ECS ListClusters", StatusFail, "AWS credentials are valid, but ECS ListClusters failed. Check region or ecs:ListClusters permission.")
	} else {
		report.Add("ECS ListClusters", StatusPass, fmt.Sprintf("%d cluster(s) visible", len(clusters)))
	}

	if opts.Cluster != "" && opts.Service != "" && opts.Task != "" {
		readiness, err := ecsProvider.CheckExecReadiness(ctx, opts.Cluster, opts.Service, opts.Task)
		if err != nil {
			report.Add("ECS Exec readiness", StatusFail, err.Error())
		} else {
			status := StatusPass
			details := "ECS Exec looks ready."
			if !readiness.ServiceExecEnabled || !readiness.TaskExecEnabled {
				status = StatusWarn
				details = strings.Join(readiness.Warnings, " ")
			}
			report.Add("ECS Exec readiness", status, details)
		}
	}

	if opts.Instance != "" {
		ec2Provider := ec2provider.New(runtime.Config, d.Runner)
		readiness, err := ec2Provider.CheckSessionReadiness(ctx, opts.Instance)
		if err != nil {
			report.Add("EC2 Session Manager readiness", StatusFail, err.Error())
		} else {
			status := StatusPass
			details := "Instance is ready for Session Manager."
			if !readiness.ManagedBySSM || strings.ToLower(readiness.PingStatus) != "online" {
				status = StatusWarn
				details = firstNonEmpty(strings.Join(readiness.Warnings, " "), "Instance is not fully ready for Session Manager.")
			}
			report.Add("EC2 Session Manager readiness", status, details)
		}
	}

	return report, nil
}

func (r *Report) Add(name string, status Status, details string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Details: details})
}

func (r *Report) String() string {
	var builder strings.Builder
	for _, check := range r.Checks {
		builder.WriteString(fmt.Sprintf("[%s] %s: %s\n", check.Status, check.Name, check.Details))
	}
	return builder.String()
}

func binaryVersion(ctx context.Context, name string, arg string) (string, error) {
	cmd := exec.CommandContext(ctx, name, arg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s is not available: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func statusFromErr(err error) Status {
	if err != nil {
		return StatusFail
	}
	return StatusPass
}

func okOrError(ok string, err error) string {
	if err != nil {
		return err.Error()
	}
	return ok
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func envProfileHint() string {
	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		return profile
	}
	return "environment/default chain"
}
