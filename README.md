# awscan

`awscan` is a Go-based terminal CLI/TUI for navigating AWS resources with minimal typing, starting with ECS Exec into running containers.

The MVP focuses on:

- `awscan doctor`
- `awscan ecs shell`
- `awscan ecs logs`
- `awscan ec2 shell`
- `awscan ec2 port-forward`

It is designed so ECS is first-class today, with clean seams for future EC2, EKS, logs, SSM, and port-forwarding workflows.

## Why awscan

The primary use case is modern AWS CLI authentication, including the newer `login_session` format in `~/.aws/config`, where tools that only expect classic profiles or old SSO layouts can fail. `awscan` handles that case explicitly by bridging through:

```bash
aws configure export-credentials --profile <profile> --format process
```

This lets the tool work with:

- standard profiles
- SSO profiles
- `credential_process`
- assume-role profiles
- environment credentials
- `login_session` compatibility via AWS CLI export

## Requirements

- Go 1.22+
- AWS CLI v2
- `session-manager-plugin`

On this machine, AWS CLI v2 and `session-manager-plugin` were present, but Go was not on `PATH`, so the project includes setup guidance instead of assuming a local compiler is already installed.

## Installation

```bash
git clone <your-repo-url>
cd awscan
make bootstrap
make build
```

The binary will be written to `./bin/awscan`.

## Usage

Open the interactive ECS shell workflow:

```bash
awscan
```

Or explicitly:

```bash
awscan ecs shell
```

Open an interactive EC2 Session Manager shell workflow:

```bash
awscan ec2 shell
```

Run EC2 shell directly with flags:

```bash
awscan ec2 shell \
  --profile default \
  --region ap-south-1 \
  --instance i-0123456789abcdef0 \
  --command /bin/bash
```

Tail ECS logs:

```bash
awscan ecs logs \
  --profile default \
  --region ap-south-1 \
  --cluster prod \
  --service api \
  --task arn:aws:ecs:ap-south-1:123456789012:task/prod/abc123 \
  --container app \
  --since 30m \
  --follow
```

Start EC2 port forward:

```bash
awscan ec2 port-forward \
  --profile default \
  --region ap-south-1 \
  --instance i-0123456789abcdef0 \
  --local-port 15432 \
  --remote-port 5432
```

Run with direct targeting flags:

```bash
awscan ecs shell \
  --profile default \
  --region ap-south-1 \
  --cluster my-cluster \
  --service my-service \
  --task 0123456789abcdef \
  --container app \
  --command /bin/sh
```

Run diagnostics:

```bash
awscan doctor
```

Run EC2-specific preflight diagnostics:

```bash
awscan doctor \
  --profile default \
  --region ap-south-1 \
  --instance i-0123456789abcdef0
```

Doctor can also preflight a known target:

```bash
awscan doctor \
  --profile default \
  --region ap-south-1 \
  --cluster arn:aws:ecs:ap-south-1:123456789012:cluster/prod \
  --service arn:aws:ecs:ap-south-1:123456789012:service/prod/api \
  --task arn:aws:ecs:ap-south-1:123456789012:task/prod/abc123
```

## Config

Preferences are stored at:

```text
~/.config/awscan/config.yaml
```

Example:

```yaml
default_profile: default
default_region: ap-south-1
recent:
  ecs:
    cluster: arn:aws:ecs:ap-south-1:123456789012:cluster/prod
    service: arn:aws:ecs:ap-south-1:123456789012:service/prod/api
    container: app
  ec2:
    instance_id: i-0123456789abcdef0
default_shells:
  app: /bin/sh
  i-0123456789abcdef0: /bin/bash
```

## Credential behavior

Resolution precedence:

1. CLI flags
2. Environment variables
3. `~/.config/awscan/config.yaml`
4. AWS shared config and credentials files
5. Safe SDK defaults with EC2 IMDS disabled for local use

If a profile includes `login_session`, `awscan` does not rely on the SDK to interpret it directly. Instead it refreshes credentials through the AWS CLI and caches them in memory until expiration.

## Architecture

- `cmd/`: Cobra commands
- `internal/aws/`: profile parsing, credential loading, caller identity, region handling
- `internal/providers/ecs/`: ECS listing, readiness checks, exec handoff
- `internal/providers/ec2/`: EC2 discovery, Session Manager readiness checks, shell handoff
- `internal/providers/ec2/`: EC2 discovery, Session Manager readiness checks, shell handoff, port forwarding
- `internal/tui/`: Bubble Tea workflow UI
- `internal/diagnostics/`: `doctor` checks and output
- `internal/config/`: local preference file management
- `pkg/plugin/`: future provider/plugin interfaces

## Testing

Run unit tests:

```bash
make test
```

Integration tests are opt-in:

```bash
AWSCAN_INTEGRATION=1 go test ./...
```

## Troubleshooting

See [TROUBLESHOOTING.md](/Users/anupgiri/Anup/awscan/TROUBLESHOOTING.md).
