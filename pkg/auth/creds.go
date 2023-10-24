package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
)

type CredentialsConfig struct {
	AuthType string

	Profile string

	AccessKey    string
	SecretKey    string
	SessionToken string

	AssumeRoleARN string
	ExternalID    string
	Region        string
	Endpoint      string
}

func GetCredsFromConfig(cfg CredentialsConfig) (*credentials.Credentials, error) {
	authType, err := validateAuthType(cfg)
	if err != nil {
		return nil, err
	}

	return getCredsByAuthType(authType, cfg)
}

func validateAuthType(cfg CredentialsConfig) (AuthType, error) {
	authSettings := ReadAuthSettingsFromEnvironmentVariables()

	if cfg.AssumeRoleARN != "" && !authSettings.AssumeRoleEnabled {
		return 0, fmt.Errorf("attempting to use assume role (ARN) for SigV4 which is not enabled")
	}

	at, err := ToAuthType(cfg.AuthType)
	if err != nil {
		return 0, err
	}

	authTypeAllowed := false
	for _, provider := range authSettings.AllowedAuthProviders {
		if provider == cfg.AuthType {
			authTypeAllowed = true
			break
		}
	}

	if !authTypeAllowed {
		return 0, fmt.Errorf("attempting to use an auth type for SigV4 that is not allowed: %q", cfg.AuthType)
	}

	return at, nil
}

func getCredsByAuthType(authType AuthType, cfg CredentialsConfig) (*credentials.Credentials, error) {
	switch authType {
	case AuthTypeKeys:
		return getCredsByKeys(cfg)
	case AuthTypeSharedCreds:
		return getCredsFromSharedCredsFile(cfg)
	case AuthTypeGrafanaAssumeRole:
		return getCredsWithGrafanaAssumeRole(cfg)
	case AuthTypeEC2IAMRole:
		return getCredsWithEC2IAMRole(cfg)
	case AuthTypeDefault:
		return getCredsWithSDKDefault(cfg)
	default:
		// I can't think of a reaason why we'd want to support a default fall-through behavior like this
		// but to prevent breaking changes, we can keep it for now
		// in the future it would be great if we can force users to select an authType
		if cfg.AssumeRoleARN != "" {
			return getCredsWithSDKDefault(cfg)
		}
		return nil, fmt.Errorf("invalid SigV4 auth type")
	}
}

func getCredsByKeys(cfg CredentialsConfig) (*credentials.Credentials, error) {
	primaryCreds := credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken)
	if cfg.AssumeRoleARN != "" {
		return assumeRole(primaryCreds, cfg.AssumeRoleARN, cfg.ExternalID, cfg.Region, cfg.Endpoint)
	}
	return primaryCreds, nil
}

func getCredsFromSharedCredsFile(cfg CredentialsConfig) (*credentials.Credentials, error) {
	primaryCreds := credentials.NewSharedCredentials("", cfg.Profile)
	if cfg.AssumeRoleARN != "" {
		return assumeRole(primaryCreds, cfg.AssumeRoleARN, cfg.ExternalID, cfg.Region, cfg.Endpoint)
	}
	return primaryCreds, nil
}

// Grafana Cloud only feature, under feature toggle awsDatasourcesTempCredentials
func getCredsWithGrafanaAssumeRole(cfg CredentialsConfig) (*credentials.Credentials, error) {
	primaryCreds := credentials.NewSharedCredentials(CredentialsPath, ProfileName)
	if cfg.AssumeRoleARN != "" {
		return assumeRole(primaryCreds, cfg.AssumeRoleARN, os.Getenv(GrafanaAssumeRoleExternalIdKeyName), cfg.Region, cfg.Endpoint)
	}
	return primaryCreds, nil
}

// Used primarily by AMG
func getCredsWithEC2IAMRole(cfg CredentialsConfig) (*credentials.Credentials, error) {
	s, err := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.Region),
		Endpoint: aws.String(cfg.Endpoint),
	})
	if err != nil {
		return nil, err
	}

	c := credentials.NewCredentials(defaults.RemoteCredProvider(*s.Config, s.Handlers))

	if cfg.AssumeRoleARN != "" {
		s, err = session.NewSession(&aws.Config{
			CredentialsChainVerboseErrors: aws.Bool(true),
			Region:                        aws.String(cfg.Region),
			Credentials:                   c,
			Endpoint:                      aws.String(cfg.Endpoint),
		})
		if err != nil {
			return nil, err
		}
		c = stscreds.NewCredentials(s, cfg.AssumeRoleARN)
	}

	return c, nil
}

// We do not specify keys to be assumed, instead it gets them from the SDK default credential chain
// See "Specifying Credentials": https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html
// Particularly useful for attaching IAM Roles to EC2 instances
func getCredsWithSDKDefault(cfg CredentialsConfig) (*credentials.Credentials, error) {
	s, err := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.Region),
		Endpoint: aws.String(cfg.Endpoint),
	})
	if err != nil {
		return nil, err
	}

	if cfg.AssumeRoleARN != "" {
		return assumeRole(s.Config.Credentials, cfg.AssumeRoleARN, cfg.ExternalID, cfg.Region, cfg.Endpoint)
	}

	return s.Config.Credentials, nil
}

func assumeRole(primaryCreds *credentials.Credentials, arn string, externalId string, region string, endpoint string) (*credentials.Credentials, error) {
	// create a session with the primary credentials
	s, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: primaryCreds,
		Endpoint:    aws.String(endpoint),
	},
	)
	if err != nil {
		return nil, err
	}

	// assume role with the primary credentials
	duration := stscreds.DefaultDuration
	expiration := time.Now().UTC().Add(duration)
	stsc := stscreds.NewCredentials(s, arn, func(p *stscreds.AssumeRoleProvider) {
		// Not sure if this is necessary, overlaps with p.Duration and is undocumented
		p.Expiry.SetExpiration(expiration, 0)
		p.Duration = duration
		if externalId != "" {
			p.ExternalID = aws.String(externalId)
		}
	})
	return stsc, nil
}
