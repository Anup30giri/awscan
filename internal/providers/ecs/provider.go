package ecs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	internalexec "github.com/anupgiri/awscan/internal/exec"
)

type Cluster struct {
	Arn  string
	Name string
}

type Service struct {
	Arn          string
	Name         string
	DesiredCount int32
	RunningCount int32
	PendingCount int32
	ExecEnabled  bool
}

type Task struct {
	Arn           string
	ShortID       string
	LastStatus    string
	DesiredStatus string
	LaunchType    string
	StartedAt     time.Time
}

type Container struct {
	Name       string
	RuntimeID  string
	LastStatus string
	ExitCode   *int32
	Reason     string
}

type TaskDetail struct {
	Task       Task
	Containers []Container
	Raw        ecstypes.Task
}

type ExecuteCommandInput struct {
	Profile       string
	Region        string
	ClusterArn    string
	TaskArn       string
	ContainerName string
	Command       string
	Interactive   bool
}

type ExecReadiness struct {
	ServiceExecEnabled bool
	TaskExecEnabled    bool
	ContainerCount     int
	Warnings           []string
}

type Provider interface {
	ListClusters(ctx context.Context) ([]Cluster, error)
	ListServices(ctx context.Context, clusterArn string) ([]Service, error)
	ListTasks(ctx context.Context, clusterArn string, serviceArn string) ([]Task, error)
	DescribeTask(ctx context.Context, clusterArn string, taskArn string) (*TaskDetail, error)
	ListContainers(ctx context.Context, task *TaskDetail) ([]Container, error)
	CheckExecReadiness(ctx context.Context, clusterArn string, serviceArn string, taskArn string) (*ExecReadiness, error)
	ExecuteCommand(ctx context.Context, input ExecuteCommandInput) error
}

type ecsAPI interface {
	ListClusters(ctx context.Context, params *awsecs.ListClustersInput, optFns ...func(*awsecs.Options)) (*awsecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *awsecs.ListServicesInput, optFns ...func(*awsecs.Options)) (*awsecs.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *awsecs.DescribeServicesInput, optFns ...func(*awsecs.Options)) (*awsecs.DescribeServicesOutput, error)
	ListTasks(ctx context.Context, params *awsecs.ListTasksInput, optFns ...func(*awsecs.Options)) (*awsecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *awsecs.DescribeTasksInput, optFns ...func(*awsecs.Options)) (*awsecs.DescribeTasksOutput, error)
}

type ServiceProvider struct {
	client  ecsAPI
	runner  internalexec.Runner
	profile string
	region  string
}

func New(cfg sdkaws.Config, profile, region string, runner internalexec.Runner) *ServiceProvider {
	return &ServiceProvider{
		client:  awsecs.NewFromConfig(cfg),
		runner:  runner,
		profile: profile,
		region:  region,
	}
}

func NewWithClient(client ecsAPI, profile, region string, runner internalexec.Runner) *ServiceProvider {
	return &ServiceProvider{
		client:  client,
		runner:  runner,
		profile: profile,
		region:  region,
	}
}

func (p *ServiceProvider) ListClusters(ctx context.Context) ([]Cluster, error) {
	paginator := awsecs.NewListClustersPaginator(p.client, &awsecs.ListClustersInput{})
	var clusters []Cluster

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list ecs clusters: %w", err)
		}

		for _, arn := range page.ClusterArns {
			clusters = append(clusters, Cluster{
				Arn:  arn,
				Name: resourceNameFromARN(arn),
			})
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	return clusters, nil
}

func (p *ServiceProvider) ListServices(ctx context.Context, clusterArn string) ([]Service, error) {
	paginator := awsecs.NewListServicesPaginator(p.client, &awsecs.ListServicesInput{
		Cluster: sdkaws.String(clusterArn),
	})

	var serviceArns []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list ecs services for cluster %q: %w", clusterArn, err)
		}
		serviceArns = append(serviceArns, page.ServiceArns...)
	}

	if len(serviceArns) == 0 {
		return []Service{}, nil
	}

	output, err := p.client.DescribeServices(ctx, &awsecs.DescribeServicesInput{
		Cluster:  sdkaws.String(clusterArn),
		Services: serviceArns,
	})
	if err != nil {
		return nil, fmt.Errorf("describe ecs services for cluster %q: %w", clusterArn, err)
	}

	services := make([]Service, 0, len(output.Services))
	for _, svc := range output.Services {
		services = append(services, Service{
			Arn:          stringValue(svc.ServiceArn),
			Name:         stringValue(svc.ServiceName),
			DesiredCount: svc.DesiredCount,
			RunningCount: svc.RunningCount,
			PendingCount: svc.PendingCount,
			ExecEnabled:  svc.EnableExecuteCommand,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, nil
}

func (p *ServiceProvider) ListTasks(ctx context.Context, clusterArn string, serviceArn string) ([]Task, error) {
	serviceName := resourceNameFromARN(serviceArn)
	input := &awsecs.ListTasksInput{
		Cluster:       sdkaws.String(clusterArn),
		DesiredStatus: ecstypes.DesiredStatusRunning,
		ServiceName:   sdkaws.String(serviceName),
	}

	paginator := awsecs.NewListTasksPaginator(p.client, input)
	var taskArns []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list running tasks for service %q: %w", serviceName, err)
		}
		taskArns = append(taskArns, page.TaskArns...)
	}

	if len(taskArns) == 0 {
		return []Task{}, nil
	}

	output, err := p.client.DescribeTasks(ctx, &awsecs.DescribeTasksInput{
		Cluster: sdkaws.String(clusterArn),
		Tasks:   taskArns,
	})
	if err != nil {
		return nil, fmt.Errorf("describe tasks for service %q: %w", serviceName, err)
	}

	tasks := make([]Task, 0, len(output.Tasks))
	for _, task := range output.Tasks {
		tasks = append(tasks, mapTask(task))
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt.After(tasks[j].StartedAt)
	})

	return tasks, nil
}

func (p *ServiceProvider) DescribeTask(ctx context.Context, clusterArn string, taskArn string) (*TaskDetail, error) {
	output, err := p.client.DescribeTasks(ctx, &awsecs.DescribeTasksInput{
		Cluster: sdkaws.String(clusterArn),
		Tasks:   []string{taskArn},
	})
	if err != nil {
		return nil, fmt.Errorf("describe task %q: %w", taskArn, err)
	}
	if len(output.Tasks) == 0 {
		return nil, fmt.Errorf("task %q was not found", taskArn)
	}

	task := output.Tasks[0]
	containers := make([]Container, 0, len(task.Containers))
	for _, container := range task.Containers {
		containers = append(containers, Container{
			Name:       stringValue(container.Name),
			RuntimeID:  stringValue(container.RuntimeId),
			LastStatus: stringValue(container.LastStatus),
			ExitCode:   container.ExitCode,
			Reason:     stringValue(container.Reason),
		})
	}

	return &TaskDetail{
		Task:       mapTask(task),
		Containers: containers,
		Raw:        task,
	}, nil
}

func (p *ServiceProvider) ListContainers(ctx context.Context, task *TaskDetail) ([]Container, error) {
	if task == nil {
		return nil, fmt.Errorf("task detail is required")
	}
	return task.Containers, nil
}

func (p *ServiceProvider) CheckExecReadiness(ctx context.Context, clusterArn string, serviceArn string, taskArn string) (*ExecReadiness, error) {
	readiness := &ExecReadiness{}

	serviceOutput, err := p.client.DescribeServices(ctx, &awsecs.DescribeServicesInput{
		Cluster:  sdkaws.String(clusterArn),
		Services: []string{serviceArn},
	})
	if err != nil {
		return nil, fmt.Errorf("describe service exec readiness: %w", err)
	}

	if len(serviceOutput.Services) > 0 {
		readiness.ServiceExecEnabled = serviceOutput.Services[0].EnableExecuteCommand
		if !readiness.ServiceExecEnabled {
			readiness.Warnings = append(readiness.Warnings, "This service does not have ECS Exec enabled.")
		}
	}

	taskDetail, err := p.DescribeTask(ctx, clusterArn, taskArn)
	if err != nil {
		return nil, err
	}

	readiness.TaskExecEnabled = taskDetail.Raw.EnableExecuteCommand
	readiness.ContainerCount = len(taskDetail.Containers)
	if !readiness.TaskExecEnabled {
		readiness.Warnings = append(readiness.Warnings, "This task does not report ECS Exec as enabled.")
	}

	return readiness, nil
}

func (p *ServiceProvider) ExecuteCommand(ctx context.Context, input ExecuteCommandInput) error {
	if err := internalexec.EnsureBinary(p.runner, "aws"); err != nil {
		return err
	}
	if err := internalexec.EnsureBinary(p.runner, "session-manager-plugin"); err != nil {
		return fmt.Errorf("session-manager-plugin is missing. Install it before using ECS Exec")
	}

	args := []string{
		"ecs", "execute-command",
		"--cluster", input.ClusterArn,
		"--task", input.TaskArn,
		"--container", input.ContainerName,
		"--command", input.Command,
	}

	if input.Interactive {
		args = append(args, "--interactive")
	} else {
		args = append(args, "--non-interactive")
	}
	if input.Region != "" {
		args = append(args, "--region", input.Region)
	}
	if input.Profile != "" {
		args = append(args, "--profile", input.Profile)
	}

	return p.runner.RunInteractive(ctx, "aws", args, nil)
}

func mapTask(task ecstypes.Task) Task {
	startedAt := time.Time{}
	if task.StartedAt != nil {
		startedAt = *task.StartedAt
	}

	return Task{
		Arn:           stringValue(task.TaskArn),
		ShortID:       resourceNameFromARN(stringValue(task.TaskArn)),
		LastStatus:    stringValue(task.LastStatus),
		DesiredStatus: stringValue(task.DesiredStatus),
		LaunchType:    string(task.LaunchType),
		StartedAt:     startedAt,
	}
}

func resourceNameFromARN(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
