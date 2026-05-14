package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func GetCallerIdentity(ctx context.Context, runtime Runtime) (*Identity, error) {
	client := sts.NewFromConfig(runtime.Config)
	output, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("get caller identity: %w", err)
	}

	return &Identity{
		Account: stringValue(output.Account),
		ARN:     stringValue(output.Arn),
		UserID:  stringValue(output.UserId),
	}, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
