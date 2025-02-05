package awsauth

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// GetAWSConfig returns an aws.Config struct initialized from the given Settings
func GetAWSConfig(ctx context.Context, authSettings Settings) (aws.Config, error) {
	return getAWSConfigWithClient(ctx, authSettings, realAWSClient{})
}

func getAWSConfigWithClient(ctx context.Context, authSettings Settings, client AWSAPIClient) (aws.Config, error) {
	logger := backend.Logger.FromContext(ctx)
	options := authSettings.BaseOptions()

	authType := authSettings.GetAuthType()
	logger.Debug(fmt.Sprintf("Using auth type: %s", authType))
	switch authType {
	case AuthTypeDefault: // nothing else to do here
	case AuthTypeKeys:
		options = append(options, authSettings.WithStaticCredentials(client))
	case AuthTypeSharedCreds, AuthTypeGrafanaAssumeRole:
		options = append(options, authSettings.WithSharedCredentials())
	case AuthTypeEC2IAMRole:
		// TODO: test this
		options = append(options, authSettings.WithEC2RoleCredentials(client))
	default:
		return aws.Config{}, fmt.Errorf("unknown auth type: %s", authType)
	}

	cfg, err := client.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, err
	}

	if authSettings.AssumeRoleARN != "" {
		options = append(authSettings.BaseOptions(), authSettings.WithAssumeRole(cfg, client))
		cfg, err = client.LoadDefaultConfig(ctx, options...)
		if err != nil {
			return aws.Config{}, err
		}
	}

	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("error retrieving credentials: %w", err)
	}

	return cfg, nil
}
