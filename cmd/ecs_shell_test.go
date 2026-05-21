package cmd

import (
	"context"
	"errors"
	"testing"

	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
)

type fakeECSProvider struct {
	commands []string
	failBash bool
}

func (f *fakeECSProvider) ListClusters(ctx context.Context) ([]ecsprovider.Cluster, error) {
	return nil, nil
}
func (f *fakeECSProvider) ListServices(ctx context.Context, clusterArn string) ([]ecsprovider.Service, error) {
	return nil, nil
}
func (f *fakeECSProvider) ListTasks(ctx context.Context, clusterArn string, serviceArn string) ([]ecsprovider.Task, error) {
	return nil, nil
}
func (f *fakeECSProvider) DescribeTask(ctx context.Context, clusterArn string, taskArn string) (*ecsprovider.TaskDetail, error) {
	return nil, nil
}
func (f *fakeECSProvider) ListContainers(ctx context.Context, task *ecsprovider.TaskDetail) ([]ecsprovider.Container, error) {
	return nil, nil
}
func (f *fakeECSProvider) CheckExecReadiness(ctx context.Context, clusterArn string, serviceArn string, taskArn string) (*ecsprovider.ExecReadiness, error) {
	return nil, nil
}
func (f *fakeECSProvider) DescribeService(ctx context.Context, clusterArn string, serviceArn string) (*ecsprovider.ServiceDetail, error) {
	return nil, nil
}
func (f *fakeECSProvider) ListServiceEvents(ctx context.Context, clusterArn string, serviceArn string) ([]ecsprovider.ServiceEvent, error) {
	return nil, nil
}
func (f *fakeECSProvider) ForceNewDeployment(ctx context.Context, clusterArn string, serviceArn string) error {
	return nil
}
func (f *fakeECSProvider) ResolveLatestTask(ctx context.Context, clusterArn string, serviceArn string) (*ecsprovider.Task, error) {
	return nil, nil
}
func (f *fakeECSProvider) ResolveLogTargets(ctx context.Context, clusterArn string, taskArn string) ([]ecsprovider.ContainerLogTarget, error) {
	return nil, nil
}
func (f *fakeECSProvider) TailLogs(ctx context.Context, input ecsprovider.TailLogsInput) error {
	return nil
}
func (f *fakeECSProvider) ExecuteCommand(ctx context.Context, input ecsprovider.ExecuteCommandInput) error {
	f.commands = append(f.commands, input.Command)
	if input.Command == "/bin/bash" && f.failBash {
		return errors.New("exec: /bin/bash: not found")
	}
	return nil
}

func TestExecuteECSShellWithFallback(t *testing.T) {
	t.Parallel()

	provider := &fakeECSProvider{failBash: true}
	err := executeECSShellWithFallback(context.Background(), provider, runtimeAdapter{profile: "default", region: "ap-south-1"}, "cluster", "task", "app", "/bin/bash")
	if err != nil {
		t.Fatalf("executeECSShellWithFallback() error = %v", err)
	}
	if len(provider.commands) != 2 || provider.commands[1] != "/bin/sh" {
		t.Fatalf("unexpected commands: %+v", provider.commands)
	}
}
