package ec2

import (
	"context"
	"testing"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type fakeRunner struct {
	name string
	args []string
}

func (r *fakeRunner) LookPath(name string) (string, error) { return "/usr/bin/" + name, nil }
func (r *fakeRunner) RunInteractive(ctx context.Context, name string, args []string, env []string) error {
	r.name = name
	r.args = append([]string{}, args...)
	return nil
}

type fakeEC2API struct{}
type fakeSSMAPI struct{}

func (fakeEC2API) DescribeInstances(ctx context.Context, params *awsec2.DescribeInstancesInput, optFns ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
	id := "i-1234567890"
	nameKey := "Name"
	nameVal := "api-node"
	privateIP := "10.0.0.10"
	publicIP := "1.2.3.4"
	az := "ap-south-1a"
	subnetID := "subnet-123"
	vpcID := "vpc-123"
	arn := "arn:aws:iam::123:instance-profile/demo"
	return &awsec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{{
				InstanceId:       &id,
				PrivateIpAddress: &privateIP,
				PublicIpAddress:  &publicIP,
				PlatformDetails:  sdkaws.String("Linux/UNIX"),
				Architecture:     ec2types.ArchitectureValuesX8664,
				InstanceType:     ec2types.InstanceTypeT3Micro,
				State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
				Placement:        &ec2types.Placement{AvailabilityZone: &az},
				SubnetId:         &subnetID,
				VpcId:            &vpcID,
				IamInstanceProfile: &ec2types.IamInstanceProfile{
					Arn: &arn,
				},
				SecurityGroups: []ec2types.GroupIdentifier{{
					GroupName: sdkaws.String("default"),
					GroupId:   sdkaws.String("sg-123"),
				}},
				Tags: []ec2types.Tag{{
					Key:   &nameKey,
					Value: &nameVal,
				}},
			}},
		}},
	}, nil
}

func (fakeSSMAPI) DescribeInstanceInformation(ctx context.Context, params *awsssm.DescribeInstanceInformationInput, optFns ...func(*awsssm.Options)) (*awsssm.DescribeInstanceInformationOutput, error) {
	id := "i-1234567890"
	return &awsssm.DescribeInstanceInformationOutput{
		InstanceInformationList: []ssmtypes.InstanceInformation{{
			InstanceId: &id,
			PingStatus: ssmtypes.PingStatusOnline,
		}},
	}, nil
}

func (fakeSSMAPI) ListDocuments(ctx context.Context, params *awsssm.ListDocumentsInput, optFns ...func(*awsssm.Options)) (*awsssm.ListDocumentsOutput, error) {
	name := "AWS-RunShellScript"
	owner := "Amazon"
	return &awsssm.ListDocumentsOutput{
		DocumentIdentifiers: []ssmtypes.DocumentIdentifier{{
			Name:         &name,
			Owner:        &owner,
			DocumentType: ssmtypes.DocumentTypeCommand,
		}},
	}, nil
}

func (fakeSSMAPI) SendCommand(ctx context.Context, params *awsssm.SendCommandInput, optFns ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
	commandID := "cmd-123"
	return &awsssm.SendCommandOutput{
		Command: &ssmtypes.Command{
			CommandId: &commandID,
		},
	}, nil
}

func TestListInstances(t *testing.T) {
	t.Parallel()

	provider := NewWithClients(fakeEC2API{}, fakeSSMAPI{}, &fakeRunner{})
	instances, err := provider.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("ListInstances() error = %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("len(instances) = %d, want 1", len(instances))
	}
	if instances[0].Name != "api-node" || !instances[0].ManagedBySSM {
		t.Fatalf("unexpected instance: %+v", instances[0])
	}
}

func TestStartSessionBuildsCLIInvocation(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	provider := NewWithClients(fakeEC2API{}, fakeSSMAPI{}, runner)
	err := provider.StartSession(context.Background(), StartSessionInput{
		Profile:    "default",
		Region:     "ap-south-1",
		InstanceID: "i-1234567890",
		Command:    "/bin/bash",
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	if runner.name != "aws" {
		t.Fatalf("runner.name = %q, want aws", runner.name)
	}
}

func TestDescribeInstance(t *testing.T) {
	t.Parallel()

	provider := NewWithClients(fakeEC2API{}, fakeSSMAPI{}, &fakeRunner{})
	detail, err := provider.DescribeInstance(context.Background(), "i-1234567890")
	if err != nil {
		t.Fatalf("DescribeInstance() error = %v", err)
	}
	if detail.InstanceType == "" {
		t.Fatal("expected instance type to be populated")
	}
}

func TestListSSMDocuments(t *testing.T) {
	t.Parallel()

	provider := NewWithClients(fakeEC2API{}, fakeSSMAPI{}, &fakeRunner{})
	docs, err := provider.ListSSMDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListSSMDocuments() error = %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one document")
	}
}

func TestSendDocumentCommand(t *testing.T) {
	t.Parallel()

	provider := NewWithClients(fakeEC2API{}, fakeSSMAPI{}, &fakeRunner{})
	commandID, err := provider.SendDocumentCommand(context.Background(), SendDocumentCommandInput{
		InstanceID: "i-1234567890",
		Document:   "AWS-RunShellScript",
		Commands:   []string{"echo hi"},
	})
	if err != nil {
		t.Fatalf("SendDocumentCommand() error = %v", err)
	}
	if commandID != "cmd-123" {
		t.Fatalf("commandID = %q, want cmd-123", commandID)
	}
}
