package awsauth

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
)

// Settings carries configuration for authenticating with AWS
type Settings struct {
	AuthType AuthType
	// deprecated: use AuthType instead
	LegacyAuthType     awsds.AuthType
	AccessKey          string
	SecretKey          string
	Region             string
	CredentialsPath    string
	CredentialsProfile string
	AssumeRoleARN      string
	Endpoint           string
	ExternalID         string
	ProxyOptions       *proxy.Options
}

func (s Settings) GetAuthType() AuthType {
	if s.AuthType != AuthTypeMissing {
		return s.AuthType
	}
	return fromLegacy(s.LegacyAuthType)
}

func (s Settings) BaseOptions() []LoadOptionsFunc {
	return []LoadOptionsFunc{s.WithRegion(), s.WithEndpoint(), s.WithProxy()}
}

func (s Settings) WithRegion() LoadOptionsFunc {
	return func(opts *config.LoadOptions) error {
		if s.Region != "" && s.Region != "default" {
			opts.Region = s.Region
		}
		return nil
	}
}

func (s Settings) WithEndpoint() LoadOptionsFunc {
	return func(options *config.LoadOptions) error {
		if s.Endpoint != "" && s.Endpoint != "default" {
			options.BaseEndpoint = s.Endpoint
		}
		return nil
	}
}

func (s Settings) WithStaticCredentials(client AWSAPIClient) LoadOptionsFunc {
	return func(opts *config.LoadOptions) error {
		opts.Credentials = client.NewStaticCredentialsProvider(s.AccessKey, s.SecretKey, "")
		return nil
	}
}

// WithSharedCredentials returns a LoadOptionsFunc to initialize config from a credentials file
func (s Settings) WithSharedCredentials() LoadOptionsFunc {
	profile := s.CredentialsProfile
	if s.GetAuthType() == AuthTypeGrafanaAssumeRole {
		profile = "assume_role_credentials"
	}
	return func(options *config.LoadOptions) error {
		options.SharedConfigProfile = profile
		if s.CredentialsPath != "" {
			options.SharedCredentialsFiles = []string{s.CredentialsPath}
		}
		return nil
	}
}

func (s Settings) WithAssumeRole(cfg aws.Config, client AWSAPIClient) LoadOptionsFunc {
	stsClient := client.NewSTSClientFromConfig(cfg)
	provider := client.NewAssumeRoleProvider(stsClient, s.AssumeRoleARN, func(options *stscreds.AssumeRoleOptions) {
		if s.ExternalID != "" {
			options.ExternalID = aws.String(s.ExternalID)
		}
	})
	cache := client.NewCredentialsCache(provider)
	return func(options *config.LoadOptions) error {
		options.Credentials = cache
		return nil
	}
}

func (s Settings) WithEC2RoleCredentials(client AWSAPIClient) LoadOptionsFunc {
	return func(options *config.LoadOptions) error {
		options.Credentials = client.NewEC2RoleCreds()
		return nil
	}
}

func (s Settings) WithProxy() LoadOptionsFunc {
	if s.ProxyOptions == nil {
		return func(*config.LoadOptions) error { return nil }
	}
	return func(options *config.LoadOptions) error {
		if client, ok := options.HTTPClient.(*http.Client); ok {
			if client.Transport == nil {
				client.Transport = http.DefaultTransport
			} else if _, ok := client.Transport.(*http.Transport); !ok {
				return fmt.Errorf("cfg.HTTPClient.Transport is not *http.Transport")
			}
			err := proxy.New(s.ProxyOptions).ConfigureSecureSocksHTTPProxy(client.Transport.(*http.Transport))
			if err != nil {
				return fmt.Errorf("error configuring Secure Socks proxy for Transport: %w", err)
			}
			return nil
		} else {
			return fmt.Errorf("cfg.HTTPClient is not *http.Client")
		}
	}
}
