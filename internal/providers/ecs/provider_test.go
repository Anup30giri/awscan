package ecs

import (
	"context"
	"testing"
	"time"

	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type fakeRunner struct{}

func (fakeRunner) LookPath(name string) (string, error) { return "/usr/bin/" + name, nil }
func (fakeRunner) RunInteractive(ctx context.Context, name string, args []string, env []string) error {
	return nil
}

type fakeAPI struct{}

func (fakeAPI) ListClusters(ctx context.Context, params *awsecs.ListClustersInput, optFns ...func(*awsecs.Options)) (*awsecs.ListClustersOutput, error) {
	return &awsecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-south-1:123:cluster/prod"},
	}, nil
}

func (fakeAPI) ListServices(ctx context.Context, params *awsecs.ListServicesInput, optFns ...func(*awsecs.Options)) (*awsecs.ListServicesOutput, error) {
	return &awsecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-south-1:123:service/prod/api"},
	}, nil
}

func (fakeAPI) DescribeServices(ctx context.Context, params *awsecs.DescribeServicesInput, optFns ...func(*awsecs.Options)) (*awsecs.DescribeServicesOutput, error) {
	name := "api"
	arn := "arn:aws:ecs:ap-south-1:123:service/prod/api"
	desired := int32(2)
	running := int32(2)
	pending := int32(0)
	return &awsecs.DescribeServicesOutput{
		Services: []ecstypes.Service{{
			ServiceArn:           &arn,
			ServiceName:          &name,
			DesiredCount:         desired,
			RunningCount:         running,
			PendingCount:         pending,
			EnableExecuteCommand: true,
		}},
	}, nil
}

func (fakeAPI) ListTasks(ctx context.Context, params *awsecs.ListTasksInput, optFns ...func(*awsecs.Options)) (*awsecs.ListTasksOutput, error) {
	_ = params
	return &awsecs.ListTasksOutput{
		TaskArns: []string{"arn:aws:ecs:ap-south-1:123:task/prod/abc123"},
	}, nil
}

func (fakeAPI) DescribeTasks(ctx context.Context, params *awsecs.DescribeTasksInput, optFns ...func(*awsecs.Options)) (*awsecs.DescribeTasksOutput, error) {
	taskArn := "arn:aws:ecs:ap-south-1:123:task/prod/abc123"
	lastStatus := "RUNNING"
	desiredStatus := "RUNNING"
	containerName := "app"
	containerStatus := "RUNNING"
	runtimeID := "runtime"
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	return &awsecs.DescribeTasksOutput{
		Tasks: []ecstypes.Task{{
			TaskArn:              &taskArn,
			LastStatus:           &lastStatus,
			DesiredStatus:        &desiredStatus,
			LaunchType:           ecstypes.LaunchTypeFargate,
			StartedAt:            &now,
			EnableExecuteCommand: true,
			Containers: []ecstypes.Container{{
				Name:       &containerName,
				LastStatus: &containerStatus,
				RuntimeId:  &runtimeID,
			}},
		}},
	}, nil
}

func TestListClusters(t *testing.T) {
	t.Parallel()

	provider := NewWithClient(fakeAPI{}, "", "", fakeRunner{})
	clusters, err := provider.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters() error = %v", err)
	}
	if len(clusters) != 1 || clusters[0].Name != "prod" {
		t.Fatalf("unexpected clusters: %+v", clusters)
	}
}

func TestCheckExecReadiness(t *testing.T) {
	t.Parallel()

	provider := NewWithClient(fakeAPI{}, "", "", fakeRunner{})
	readiness, err := provider.CheckExecReadiness(context.Background(), "cluster", "service", "task")
	if err != nil {
		t.Fatalf("CheckExecReadiness() error = %v", err)
	}
	if !readiness.ServiceExecEnabled || !readiness.TaskExecEnabled {
		t.Fatalf("expected exec readiness, got %+v", readiness)
	}
}

func TestExecuteCommandBuildsCLIInvocation(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	provider := NewWithClient(fakeAPI{}, "default", "ap-south-1", runner)

	err := provider.ExecuteCommand(context.Background(), ExecuteCommandInput{
		Profile:       "default",
		Region:        "ap-south-1",
		ClusterArn:    "cluster",
		TaskArn:       "task",
		ContainerName: "app",
		Command:       "/bin/sh",
		Interactive:   true,
	})
	if err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	if runner.name != "aws" {
		t.Fatalf("runner.name = %q, want aws", runner.name)
	}
}

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) LookPath(name string) (string, error) { return "/usr/bin/" + name, nil }
func (r *recordingRunner) RunInteractive(ctx context.Context, name string, args []string, env []string) error {
	r.name = name
	r.args = append([]string{}, args...)
	return nil
}
