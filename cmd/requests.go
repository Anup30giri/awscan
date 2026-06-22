package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	internalaws "github.com/anupgiri/awscan/internal/aws"
	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
)

type ECSShellRequest struct {
	Profile   string
	Region    string
	Cluster   string
	Service   string
	Task      string
	Container string
	Command   string
}

type ECSLogsRequest struct {
	Profile       string
	Region        string
	Cluster       string
	Service       string
	Task          string
	Container     string
	AllContainers bool
	Follow        bool
	Since         string
}

type EC2ShellRequest struct {
	Profile  string
	Region   string
	Instance string
	Command  string
}

type EC2PortForwardRequest struct {
	Profile    string
	Region     string
	Instance   string
	RemoteHost string
	LocalPort  int
	RemotePort int
}

func resolveRuntime(ctx context.Context, env *commandEnv, profile, region string, allowPrompt bool) (internalaws.Runtime, string, string, error) {
	resolvedProfile := profile
	resolvedRegion := region

	if allowPrompt {
		selectedProfile, selectedRegion, err := resolveProfileAndRegionInteractively(ctx, env, resolvedProfile, resolvedRegion)
		if err != nil {
			return internalaws.Runtime{}, "", "", err
		}
		resolvedProfile = selectedProfile
		resolvedRegion = selectedRegion
	}

	runtime, err := env.resolver.Resolve(ctx, internalaws.ResolveOptions{
		Profile: resolvedProfile,
		Region:  resolvedRegion,
	})
	if err != nil {
		return internalaws.Runtime{}, "", "", err
	}
	if identity, identityErr := internalaws.GetCallerIdentity(ctx, runtime); identityErr == nil {
		runtime.Identity = identity
	}

	return runtime, runtime.Profile, runtime.Region, nil
}

func executeECSShellRequest(ctx context.Context, env *commandEnv, runtime internalaws.Runtime, provider ecsprovider.Provider, req ECSShellRequest) error {
	task := req.Task
	if strings.TrimSpace(task) == "" {
		latest, err := provider.ResolveLatestTask(ctx, req.Cluster, req.Service)
		if err != nil {
			return err
		}
		task = latest.Arn
	}

	taskDetail, err := provider.DescribeTask(ctx, req.Cluster, task)
	if err != nil {
		return err
	}

	readiness, err := provider.CheckExecReadiness(ctx, req.Cluster, req.Service, task)
	if err != nil {
		return err
	}
	if !readiness.ServiceExecEnabled || !readiness.TaskExecEnabled {
		return fmt.Errorf("this service/task does not have ECS Exec enabled. Run: aws ecs update-service --cluster %s --service %s --enable-execute-command --force-new-deployment", req.Cluster, req.Service)
	}

	if _, err := findContainer(taskDetail.Containers, req.Container); err != nil {
		return err
	}

	command := firstNonEmpty(req.Command, env.prefs.DefaultShells[req.Container], "/bin/sh")
	if err := saveECSPreferences(env, runtime.Profile, runtime.Region, req.Cluster, req.Service, req.Container, command); err != nil {
		return err
	}

	return executeECSShellWithFallback(ctx, provider, runtimeAdapter{
		profile: runtime.Profile,
		region:  runtime.Region,
		account: accountID(runtime),
	}, req.Cluster, task, req.Container, command)
}

func executeECSLogsRequest(ctx context.Context, env *commandEnv, runtime internalaws.Runtime, provider ecsprovider.Provider, req ECSLogsRequest) error {
	task := req.Task
	if strings.TrimSpace(task) == "" {
		latest, err := provider.ResolveLatestTask(ctx, req.Cluster, req.Service)
		if err != nil {
			return err
		}
		task = latest.Arn
	}

	targets, err := provider.ResolveLogTargets(ctx, req.Cluster, task)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("selected task does not expose awslogs targets. Check task definition log driver and awslogs-group/awslogs-stream-prefix settings")
	}

	logGroups := map[string]bool{}
	var logGroup string
	streams := []string{}
	for _, target := range targets {
		if !req.AllContainers && target.ContainerName != req.Container {
			continue
		}
		logGroup = target.LogGroup
		logGroups[target.LogGroup] = true
		streams = append(streams, target.LogStream)
	}
	if logGroup == "" || len(streams) == 0 {
		return fmt.Errorf("no awslogs target found for container %q on selected task", req.Container)
	}
	if req.AllContainers && len(logGroups) > 1 {
		return fmt.Errorf("all-containers mode requires all selected containers to share same CloudWatch log group")
	}

	saveGlobalPreferences(env, runtime.Profile, runtime.Region)
	return provider.TailLogs(ctx, ecsprovider.TailLogsInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		LogGroup:   logGroup,
		LogStreams: streams,
		Follow:     req.Follow,
		Since:      req.Since,
	})
}

func executeEC2ShellRequest(ctx context.Context, env *commandEnv, runtime internalaws.Runtime, provider ec2provider.Provider, req EC2ShellRequest) error {
	instanceID, err := provider.ResolveInstanceID(ctx, req.Instance)
	if err != nil {
		return err
	}

	command := firstNonEmpty(req.Command, env.prefs.DefaultShells[instanceID], "/bin/sh")

	readiness, err := provider.CheckSessionReadiness(ctx, instanceID)
	if err != nil {
		return err
	}
	if !readiness.ManagedBySSM || strings.ToLower(readiness.PingStatus) != "online" {
		return fmt.Errorf("this EC2 instance is not ready for Session Manager. Ensure SSM Agent is installed, the instance is managed by Systems Manager, and the IAM role allows SSM")
	}

	if err := saveEC2Preferences(env, runtime.Profile, runtime.Region, instanceID, command); err != nil {
		return err
	}

	return provider.StartSession(ctx, ec2provider.StartSessionInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		InstanceID: instanceID,
		Command:    command,
	})
}

func executeEC2PortForwardRequest(ctx context.Context, env *commandEnv, runtime internalaws.Runtime, provider ec2provider.Provider, req EC2PortForwardRequest) error {
	instanceID, err := provider.ResolveInstanceID(ctx, req.Instance)
	if err != nil {
		return err
	}

	if instanceID == "" || req.LocalPort <= 0 || req.RemotePort <= 0 {
		return errors.New("instance, local-port, and remote-port are required")
	}

	readiness, err := provider.CheckSessionReadiness(ctx, instanceID)
	if err != nil {
		return err
	}
	if !readiness.ManagedBySSM || strings.ToLower(readiness.PingStatus) != "online" {
		return fmt.Errorf("this EC2 instance is not ready for Session Manager. Ensure SSM Agent is installed, instance is managed by Systems Manager, and IAM role allows SSM")
	}

	saveGlobalPreferences(env, runtime.Profile, runtime.Region)
	env.prefs.Recent.EC2.InstanceID = instanceID
	env.prefs.Recent.EC2.RemoteHost = req.RemoteHost
	env.prefs.Recent.EC2.LocalPort = req.LocalPort
	env.prefs.Recent.EC2.RemotePort = req.RemotePort
	if err := env.app.Config.Save(env.prefs); err != nil {
		return err
	}

	return provider.StartPortForward(ctx, ec2provider.StartPortForwardInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		InstanceID: instanceID,
		LocalPort:  req.LocalPort,
		RemotePort: req.RemotePort,
		RemoteHost: req.RemoteHost,
	})
}
