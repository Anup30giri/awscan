package ec2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	internalexec "github.com/anupgiri/awscan/internal/exec"
)

type Instance struct {
	InstanceID     string
	Name           string
	PrivateIP      string
	PublicIP       string
	Platform       string
	State          string
	AvailabilityAZ string
	ManagedBySSM   bool
}

type SessionReadiness struct {
	InstanceFound bool
	ManagedBySSM  bool
	PingStatus    string
	Warnings      []string
}

type StartSessionInput struct {
	Profile    string
	Region     string
	InstanceID string
	Command    string
}

type Provider interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	CheckSessionReadiness(ctx context.Context, instanceID string) (*SessionReadiness, error)
	StartSession(ctx context.Context, input StartSessionInput) error
}

type ec2API interface {
	DescribeInstances(ctx context.Context, params *awsec2.DescribeInstancesInput, optFns ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error)
}

type ssmAPI interface {
	DescribeInstanceInformation(ctx context.Context, params *awsssm.DescribeInstanceInformationInput, optFns ...func(*awsssm.Options)) (*awsssm.DescribeInstanceInformationOutput, error)
}

type ServiceProvider struct {
	ec2Client ec2API
	ssmClient ssmAPI
	runner    internalexec.Runner
}

func New(cfg sdkaws.Config, runner internalexec.Runner) *ServiceProvider {
	return &ServiceProvider{
		ec2Client: awsec2.NewFromConfig(cfg),
		ssmClient: awsssm.NewFromConfig(cfg),
		runner:    runner,
	}
}

func NewWithClients(ec2Client ec2API, ssmClient ssmAPI, runner internalexec.Runner) *ServiceProvider {
	return &ServiceProvider{
		ec2Client: ec2Client,
		ssmClient: ssmClient,
		runner:    runner,
	}
}

func (p *ServiceProvider) ListInstances(ctx context.Context) ([]Instance, error) {
	paginator := awsec2.NewDescribeInstancesPaginator(p.ec2Client, &awsec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: sdkaws.String("instance-state-name"), Values: []string{"running"}},
		},
	})

	managed, _ := p.fetchManagedInstanceStatuses(ctx)
	var instances []Instance
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe running EC2 instances: %w", err)
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				instanceID := stringValue(instance.InstanceId)
				instances = append(instances, Instance{
					InstanceID:     instanceID,
					Name:           findNameTag(instance.Tags),
					PrivateIP:      stringValue(instance.PrivateIpAddress),
					PublicIP:       stringValue(instance.PublicIpAddress),
					Platform:       detectPlatform(instance),
					State:          detectState(instance),
					AvailabilityAZ: detectAvailabilityZone(instance),
					ManagedBySSM:   managed[instanceID],
				})
			}
		}
	}

	sort.Slice(instances, func(i, j int) bool {
		left := firstNonEmpty(instances[i].Name, instances[i].InstanceID)
		right := firstNonEmpty(instances[j].Name, instances[j].InstanceID)
		return left < right
	})

	return instances, nil
}

func (p *ServiceProvider) CheckSessionReadiness(ctx context.Context, instanceID string) (*SessionReadiness, error) {
	readiness := &SessionReadiness{}

	output, err := p.ec2Client.DescribeInstances(ctx, &awsec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("describe EC2 instance %q: %w", instanceID, err)
	}

	for _, reservation := range output.Reservations {
		for _, instance := range reservation.Instances {
			if stringValue(instance.InstanceId) == instanceID {
				readiness.InstanceFound = true
			}
		}
	}
	if !readiness.InstanceFound {
		return nil, fmt.Errorf("instance %q was not found", instanceID)
	}

	infoOutput, err := p.ssmClient.DescribeInstanceInformation(ctx, &awsssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{Key: sdkaws.String("InstanceIds"), Values: []string{instanceID}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("describe SSM instance information for %q: %w", instanceID, err)
	}

	if len(infoOutput.InstanceInformationList) == 0 {
		readiness.Warnings = append(readiness.Warnings, "This instance is not registered in Systems Manager.")
		return readiness, nil
	}

	info := infoOutput.InstanceInformationList[0]
	readiness.ManagedBySSM = true
	readiness.PingStatus = string(info.PingStatus)
	if info.PingStatus != ssmtypes.PingStatusOnline {
		readiness.Warnings = append(readiness.Warnings, fmt.Sprintf("SSM agent is not online (ping status: %s).", info.PingStatus))
	}

	return readiness, nil
}

func (p *ServiceProvider) StartSession(ctx context.Context, input StartSessionInput) error {
	if err := internalexec.EnsureBinary(p.runner, "aws"); err != nil {
		return err
	}
	if err := internalexec.EnsureBinary(p.runner, "session-manager-plugin"); err != nil {
		return fmt.Errorf("session-manager-plugin is missing. Install it before using EC2 shell")
	}

	args := []string{"ssm", "start-session", "--target", input.InstanceID}
	if input.Command != "" {
		args = append(args,
			"--document-name", "AWS-StartInteractiveCommand",
			"--parameters", fmt.Sprintf("command=%s", shellParameter(input.Command)),
		)
	}
	if input.Region != "" {
		args = append(args, "--region", input.Region)
	}
	if input.Profile != "" {
		args = append(args, "--profile", input.Profile)
	}

	return p.runner.RunInteractive(ctx, "aws", args, nil)
}

func (p *ServiceProvider) fetchManagedInstanceStatuses(ctx context.Context) (map[string]bool, error) {
	statuses := map[string]bool{}
	paginator := awsssm.NewDescribeInstanceInformationPaginator(p.ssmClient, &awsssm.DescribeInstanceInformationInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, info := range page.InstanceInformationList {
			statuses[stringValue(info.InstanceId)] = info.PingStatus == ssmtypes.PingStatusOnline
		}
	}
	return statuses, nil
}

func findNameTag(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if stringValue(tag.Key) == "Name" {
			return stringValue(tag.Value)
		}
	}
	return ""
}

func detectPlatform(instance ec2types.Instance) string {
	if instance.PlatformDetails != nil && strings.TrimSpace(*instance.PlatformDetails) != "" {
		return *instance.PlatformDetails
	}
	if instance.Platform == ec2types.PlatformValuesWindows {
		return "Windows"
	}
	return "Linux/Unix"
}

func detectState(instance ec2types.Instance) string {
	if instance.State != nil {
		return string(instance.State.Name)
	}
	return ""
}

func detectAvailabilityZone(instance ec2types.Instance) string {
	if instance.Placement != nil {
		return stringValue(instance.Placement.AvailabilityZone)
	}
	return ""
}

func shellParameter(command string) string {
	escaped := strings.ReplaceAll(command, `"`, `\"`)
	return fmt.Sprintf("[\"%s\"]", escaped)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
