package awsauth

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type ConfigProvider interface {
	GetConfig(context.Context, Settings) (aws.Config, error)
}

func NewConfigProvider() ConfigProvider {
	return newAWSConfigProviderWithClient(awsAPIClient{})
}

func newAWSConfigProviderWithClient(client AWSAPIClient) *awsConfigProvider {
	return &awsConfigProvider{client, make(map[uint64]aws.Config)}
}

type awsConfigProvider struct {
	client AWSAPIClient
	cache  map[uint64]aws.Config
}

func (rcp *awsConfigProvider) GetConfig(ctx context.Context, authSettings Settings) (aws.Config, error) {
	logger := backend.Logger.FromContext(ctx)

	key := authSettings.Hash()
	cached, exists := rcp.cache[key]
	if exists {
		logger.Debug("returning config from cache")
		return cached, nil
	}
	logger.Debug("creating new config")

	options := authSettings.BaseOptions()

	authType := authSettings.GetAuthType()
	logger.Debug(fmt.Sprintf("Using auth type: %s", authType))
	switch authType {
	case AuthTypeDefault: // nothing else to do here
	case AuthTypeKeys:
		options = append(options, authSettings.WithStaticCredentials(rcp.client))
	case AuthTypeSharedCreds:
		options = append(options, authSettings.WithSharedCredentials())
	case AuthTypeGrafanaAssumeRole:
		options = append(options, authSettings.WithGrafanaAssumeRole(ctx, rcp.client))
	case AuthTypeEC2IAMRole:
		// TODO: test this
		options = append(options, authSettings.WithEC2RoleCredentials(rcp.client))
	default:
		return aws.Config{}, fmt.Errorf("unknown auth type: %s", authType)
	}

	cfg, err := rcp.client.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, err
	}

	if authSettings.AssumeRoleARN != "" {
		options = append(authSettings.BaseOptions(), authSettings.WithAssumeRole(cfg, rcp.client))
		cfg, err = rcp.client.LoadDefaultConfig(ctx, options...)
		if err != nil {
			return aws.Config{}, err
		}
	}

	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("error retrieving credentials: %w", err)
	}

	rcp.cache[key] = cfg
	return cfg, nil
}
