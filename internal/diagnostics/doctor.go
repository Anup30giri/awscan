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
	Profile  string
	Region   string
	Target   string
	Check    string
	Cluster  string
	Service  string
	Task     string
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

	if opts.Target == "" || opts.Target == "ecs" {
		d.runECSChecks(ctx, report, runtime, opts)
	}
	if opts.Target == "" || opts.Target == "ec2" {
		d.runEC2Checks(ctx, report, runtime, opts)
	}

	return report, nil
}

func (d *Doctor) runECSChecks(ctx context.Context, report *Report, runtime internalaws.Runtime, opts Options) {
	ecsProvider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, d.Runner)
	clusters, err := ecsProvider.ListClusters(ctx)
	if err != nil {
		report.Add("ECS ListClusters", classifyOperationStatus(err), classifyOperationMessage("ECS ListClusters", "ecs:ListClusters", err))
		return
	}
	report.Add("ECS ListClusters", StatusPass, fmt.Sprintf("%d cluster(s) visible", len(clusters)))

	if opts.Cluster == "" || opts.Service == "" {
		return
	}

	serviceDetail, err := ecsProvider.DescribeService(ctx, opts.Cluster, opts.Service)
	if err != nil {
		report.Add("ECS DescribeService", classifyOperationStatus(err), classifyOperationMessage("ECS DescribeService", "ecs:DescribeServices", err))
		return
	}
	report.Add("ECS DescribeService", StatusPass, fmt.Sprintf("service=%s taskDef=%s exec=%t", serviceDetail.Service.Name, serviceDetail.TaskDefinitionArn, serviceDetail.Service.ExecEnabled))

	switch opts.Check {
	case "restart":
		report.Add("ECS Restart readiness", StatusPass, "service resolved and restart path available")
	case "logs":
		taskArn := opts.Task
		if taskArn == "" {
			latest, err := ecsProvider.ResolveLatestTask(ctx, opts.Cluster, opts.Service)
			if err != nil {
				report.Add("ECS LatestTask", StatusFail, err.Error())
				return
			}
			taskArn = latest.Arn
		}
		targets, err := ecsProvider.ResolveLogTargets(ctx, opts.Cluster, taskArn)
		if err != nil {
			report.Add("ECS Logs readiness", StatusFail, err.Error())
			return
		}
		if len(targets) == 0 {
			report.Add("ECS Logs readiness", StatusWarn, "task definition does not expose awslogs targets")
			return
		}
		report.Add("ECS Logs readiness", StatusPass, fmt.Sprintf("%d awslogs target(s) resolved", len(targets)))
	default:
		if opts.Task != "" {
			readiness, err := ecsProvider.CheckExecReadiness(ctx, opts.Cluster, opts.Service, opts.Task)
			if err != nil {
				report.Add("ECS Exec readiness", StatusFail, err.Error())
				return
			}
			status := StatusPass
			details := "ECS Exec looks ready."
			if !readiness.ServiceExecEnabled || !readiness.TaskExecEnabled {
				status = StatusWarn
				details = strings.Join(readiness.Warnings, " ")
			}
			report.Add("ECS Exec readiness", status, details)
		}
	}
}

func (d *Doctor) runEC2Checks(ctx context.Context, report *Report, runtime internalaws.Runtime, opts Options) {
	ec2Provider := ec2provider.New(runtime.Config, d.Runner)
	if opts.Instance == "" {
		return
	}
	readiness, err := ec2Provider.CheckSessionReadiness(ctx, opts.Instance)
	if err != nil {
		report.Add("EC2 Session Manager readiness", StatusFail, err.Error())
		return
	}
	status := StatusPass
	details := "Instance is ready for Session Manager."
	if !readiness.ManagedBySSM || strings.ToLower(readiness.PingStatus) != "online" {
		status = StatusWarn
		details = firstNonEmpty(strings.Join(readiness.Warnings, " "), "Instance is not fully ready for Session Manager.")
	}
	report.Add("EC2 Session Manager readiness", status, details)

	detail, err := ec2Provider.DescribeInstance(ctx, opts.Instance)
	if err == nil {
		report.Add("EC2 DescribeInstance", StatusPass, fmt.Sprintf("instance=%s state=%s platform=%s", detail.InstanceID, detail.State, detail.Platform))
	}
	switch opts.Check {
	case "port-forward":
		if err := binaryRequired("session-manager-plugin"); err != nil {
			report.Add("EC2 Port Forward prerequisites", StatusFail, err.Error())
		} else {
			report.Add("EC2 Port Forward prerequisites", StatusPass, "session-manager-plugin installed")
		}
	case "documents":
		docs, err := ec2Provider.ListSSMDocuments(ctx)
		if err != nil {
			report.Add("EC2 Documents readiness", StatusFail, err.Error())
		} else {
			report.Add("EC2 Documents readiness", StatusPass, fmt.Sprintf("%d allowlisted SSM document(s) available", len(docs)))
		}
	}
}

func classifyOperationStatus(err error) Status {
	if err == nil {
		return StatusPass
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "accessdenied"), strings.Contains(text, "unauthorizedoperation"), strings.Contains(text, "not authorized"):
		return StatusFail
	default:
		return StatusFail
	}
}

func classifyOperationMessage(op, permission string, err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "accessdenied"), strings.Contains(text, "unauthorizedoperation"), strings.Contains(text, "not authorized"):
		return fmt.Sprintf("%s failed with an authorization error. Check %s permission. Original error: %s", op, permission, err)
	case strings.Contains(text, "endpoint"), strings.Contains(text, "no such host"):
		return fmt.Sprintf("%s failed due to endpoint or region resolution. Check region/network. Original error: %s", op, err)
	default:
		return err.Error()
	}
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

func binaryRequired(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s is not installed or not on PATH", name)
	}
	return nil
}
