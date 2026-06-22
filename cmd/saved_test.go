package cmd

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"

	"github.com/anupgiri/awscan/internal/app"
	internalaws "github.com/anupgiri/awscan/internal/aws"
	appconfig "github.com/anupgiri/awscan/internal/config"
	"github.com/anupgiri/awscan/internal/diagnostics"
	internalexec "github.com/anupgiri/awscan/internal/exec"
	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
)

type fakeResolver struct {
	runtime internalaws.Runtime
}

func (f fakeResolver) Resolve(ctx context.Context, opts internalaws.ResolveOptions) (internalaws.Runtime, error) {
	runtime := f.runtime
	runtime.Profile = opts.Profile
	runtime.Region = opts.Region
	runtime.Config = sdkaws.Config{
		Region:      opts.Region,
		Credentials: sdkaws.AnonymousCredentials{},
	}
	return runtime, nil
}

type fakeSavedECSProvider struct {
	fakeECSProvider
	latestTask string
}

func (f *fakeSavedECSProvider) ResolveLatestTask(ctx context.Context, clusterArn string, serviceArn string) (*ecsprovider.Task, error) {
	return &ecsprovider.Task{Arn: f.latestTask, ShortID: "latest"}, nil
}

func (f *fakeSavedECSProvider) DescribeTask(ctx context.Context, clusterArn string, taskArn string) (*ecsprovider.TaskDetail, error) {
	return &ecsprovider.TaskDetail{
		Task: ecsprovider.Task{Arn: taskArn, ShortID: "latest"},
		Containers: []ecsprovider.Container{
			{Name: "app", LastStatus: "RUNNING"},
		},
	}, nil
}

func (f *fakeSavedECSProvider) CheckExecReadiness(ctx context.Context, clusterArn string, serviceArn string, taskArn string) (*ecsprovider.ExecReadiness, error) {
	return &ecsprovider.ExecReadiness{ServiceExecEnabled: true, TaskExecEnabled: true}, nil
}

func (f *fakeSavedECSProvider) ResolveLogTargets(ctx context.Context, clusterArn string, taskArn string) ([]ecsprovider.ContainerLogTarget, error) {
	return []ecsprovider.ContainerLogTarget{{
		ContainerName: "app",
		LogGroup:      "/ecs/prod-api",
		LogStream:     "ecs/app/latest",
	}}, nil
}

type fakeSavedEC2Provider struct {
	startedSession     bool
	startedPortForward bool
}

func (f *fakeSavedEC2Provider) ListInstances(ctx context.Context) ([]ec2provider.Instance, error) {
	return nil, nil
}
func (f *fakeSavedEC2Provider) ResolveInstanceID(ctx context.Context, value string) (string, error) {
	return value, nil
}
func (f *fakeSavedEC2Provider) CheckSessionReadiness(ctx context.Context, instanceID string) (*ec2provider.SessionReadiness, error) {
	return &ec2provider.SessionReadiness{ManagedBySSM: true, PingStatus: "Online"}, nil
}
func (f *fakeSavedEC2Provider) StartSession(ctx context.Context, input ec2provider.StartSessionInput) error {
	f.startedSession = true
	return nil
}
func (f *fakeSavedEC2Provider) StartPortForward(ctx context.Context, input ec2provider.StartPortForwardInput) error {
	f.startedPortForward = true
	return nil
}
func (f *fakeSavedEC2Provider) DescribeInstance(ctx context.Context, instanceID string) (*ec2provider.InstanceDetail, error) {
	return &ec2provider.InstanceDetail{Instance: ec2provider.Instance{InstanceID: instanceID}}, nil
}
func (f *fakeSavedEC2Provider) ListSSMDocuments(ctx context.Context) ([]ec2provider.DocumentInfo, error) {
	return nil, nil
}
func (f *fakeSavedEC2Provider) SendDocumentCommand(ctx context.Context, input ec2provider.SendDocumentCommandInput) (string, error) {
	return "", nil
}

func TestRunSavedWorkflowByNameDispatchesECSShell(t *testing.T) {
	t.Parallel()

	manager := appconfig.NewManagerForPath(filepath.Join(t.TempDir(), "config.yaml"))
	prefs := appconfig.DefaultPreferences()
	prefs.Saved["prod-api-shell"] = appconfig.SavedWorkflow{
		Kind:      appconfig.SavedWorkflowKindECSShell,
		Profile:   "default",
		Region:    "ap-south-1",
		Cluster:   "prod",
		Service:   "api",
		Container: "app",
		Command:   "/bin/bash",
	}
	env := &commandEnv{
		app:   &app.App{Logger: slog.Default(), Config: manager},
		prefs: prefs,
		resolver: fakeResolver{
			runtime: internalaws.Runtime{ProfileKind: internalaws.ProfileKindStandard},
		},
		runner: internalexec.NewRunner(),
	}

	provider := &fakeSavedECSProvider{latestTask: "task-latest"}
	prevFactory := savedECSProviderFactory
	savedECSProviderFactory = func(runtime internalaws.Runtime, env *commandEnv) ecsprovider.Provider { return provider }
	t.Cleanup(func() { savedECSProviderFactory = prevFactory })

	if err := runSavedWorkflowByName(context.Background(), env, &rootFlags{}, "prod-api-shell"); err != nil {
		t.Fatalf("runSavedWorkflowByName() error = %v", err)
	}
	if len(provider.commands) == 0 || provider.commands[0] != "/bin/bash" {
		t.Fatalf("expected ECS saved workflow to execute command, got %+v", provider.commands)
	}
}

func TestRunSavedWorkflowByNameProfileRegionOverride(t *testing.T) {
	t.Parallel()

	manager := appconfig.NewManagerForPath(filepath.Join(t.TempDir(), "config.yaml"))
	prefs := appconfig.DefaultPreferences()
	prefs.Saved["shell"] = appconfig.SavedWorkflow{
		Kind:     appconfig.SavedWorkflowKindEC2Shell,
		Profile:  "default",
		Region:   "ap-south-1",
		Instance: "i-123",
		Command:  "/bin/sh",
	}
	var resolved internalaws.ResolveOptions
	env := &commandEnv{
		app:   &app.App{Logger: slog.Default(), Config: manager},
		prefs: prefs,
		resolver: fakeResolver{
			runtime: internalaws.Runtime{ProfileKind: internalaws.ProfileKindStandard},
		},
		runner: internalexec.NewRunner(),
	}
	env.resolver = resolverFunc(func(ctx context.Context, opts internalaws.ResolveOptions) (internalaws.Runtime, error) {
		resolved = opts
		return fakeResolver{runtime: internalaws.Runtime{ProfileKind: internalaws.ProfileKindStandard}}.Resolve(ctx, opts)
	})

	provider := &fakeSavedEC2Provider{}
	prevFactory := savedEC2ProviderFactory
	savedEC2ProviderFactory = func(runtime internalaws.Runtime, env *commandEnv) ec2provider.Provider { return provider }
	t.Cleanup(func() { savedEC2ProviderFactory = prevFactory })

	if err := runSavedWorkflowByName(context.Background(), env, &rootFlags{profile: "prod", region: "us-east-1"}, "shell"); err != nil {
		t.Fatalf("runSavedWorkflowByName() error = %v", err)
	}
	if resolved.Profile != "prod" || resolved.Region != "us-east-1" {
		t.Fatalf("resolve opts = %+v, want profile override prod and region override us-east-1", resolved)
	}
}

func TestApplySavedWorkflowToDoctorFlags(t *testing.T) {
	t.Parallel()

	flags := diagnostics.Options{}
	applySavedWorkflowToDoctorFlags(&flags, appconfig.SavedWorkflow{
		Kind:     appconfig.SavedWorkflowKindEC2PortForward,
		Instance: "i-123",
	})
	if flags.Target != "ec2" || flags.Check != "port-forward" || flags.Instance != "i-123" {
		t.Fatalf("unexpected doctor flags: %+v", flags)
	}
}

type resolverFunc func(ctx context.Context, opts internalaws.ResolveOptions) (internalaws.Runtime, error)

func (f resolverFunc) Resolve(ctx context.Context, opts internalaws.ResolveOptions) (internalaws.Runtime, error) {
	return f(ctx, opts)
}
