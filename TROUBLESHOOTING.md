# Troubleshooting

## `session-manager-plugin` is missing

Install the Session Manager Plugin for AWS CLI:

- macOS: install via the official AWS package or Homebrew-supported distribution path
- Linux: install the official package for your distro
- Windows: install the official MSI package

Then confirm:

```bash
session-manager-plugin --version
```

## `aws credential resolution failed`

Run:

```bash
aws sts get-caller-identity
```

If that fails, refresh your login:

```bash
aws login --profile <profile>
```

or:

```bash
aws sso login --profile <profile>
```

## `login_session` profile is not being consumed by other tools

`awscan` supports this natively by calling:

```bash
aws configure export-credentials --profile <profile> --format process
```

If you need a compatibility profile for another tool, use:

```ini
[profile signin]
login_session = arn:aws:sts::ACCOUNT_ID:assumed-role/AWSReservedSSO_AdministratorAccess_xxx/user
region = ap-south-1

[profile tool]
credential_process = aws configure export-credentials --profile signin --format process
region = ap-south-1
```

## `No ECS clusters found`

This usually means one of:

- wrong region
- valid AWS identity but no ECS clusters in that account/region
- missing `ecs:ListClusters` permission

Try:

```bash
awscan doctor --profile <profile> --region <region>
```

## `No running tasks found for this service`

Check that:

- the service is in the right cluster
- tasks are actually running
- the service name is correct

You can confirm with:

```bash
aws ecs list-tasks \
  --cluster <cluster> \
  --service-name <service> \
  --desired-status RUNNING
```

## `This service/task does not have ECS Exec enabled`

Enable it on the service and redeploy:

```bash
aws ecs update-service \
  --cluster <cluster> \
  --service <service> \
  --enable-execute-command \
  --force-new-deployment
```

Also make sure the task role and execution environment support SSM messaging.

## `This EC2 instance is not ready for Session Manager`

Check that:

- the instance is running
- SSM Agent is installed and healthy
- the instance IAM role allows Session Manager
- the instance appears in Systems Manager as a managed node

You can verify with:

```bash
awscan doctor --profile <profile> --region <region> --instance <instance-id>
```

and:

```bash
aws ssm describe-instance-information \
  --filters Key=InstanceIds,Values=<instance-id>
```

## Required IAM permissions

Common permissions for the MVP:

- `ecs:ListClusters`
- `ecs:ListServices`
- `ecs:ListTasks`
- `ecs:DescribeTasks`
- `ecs:DescribeServices`
- `ecs:ExecuteCommand`
- `ssm:StartSession`
- `ssm:DescribeSessions`
- `ssm:TerminateSession`
- `ssmmessages:CreateControlChannel`
- `ssmmessages:CreateDataChannel`
- `ssmmessages:OpenControlChannel`
- `ssmmessages:OpenDataChannel`

For EC2 Session Manager workflows:

- `ec2:DescribeInstances`
- `ssm:DescribeInstanceInformation`
- `ssm:StartSession`

For future logs support:

- `logs:DescribeLogGroups`
- `logs:DescribeLogStreams`
- `logs:GetLogEvents`
