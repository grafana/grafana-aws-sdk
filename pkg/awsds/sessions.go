package awsds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/experimental/errorsource"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"

	awsV2 "github.com/aws/aws-sdk-go-v2/aws"
)

const (
	// awsTempCredsAccessKey and awsTempCredsSecretKey are the files containing the
	awsTempCredsAccessKey = "/tmp/aws.credentials/access-key-id"
	awsTempCredsSecretKey = "/tmp/aws.credentials/secret-access-key"
)

type envelope struct {
	session    *session.Session
	expiration time.Time
}

// SessionCache cache sessions for a while
type SessionCache struct {
	sessCache     map[string]envelope
	sessCacheLock sync.RWMutex
}

// NewSessionCache creates a new session cache using the default settings loaded from environment variables
func NewSessionCache() *SessionCache {
	return &SessionCache{
		sessCache: map[string]envelope{},
	}
}

const (
	// CredentialsPath is the path to the shared credentials file in the instance for the aws/aws-sdk
	// if empty string, the path is ~/.aws/credentials
	CredentialsPath = ""

	// ProfileName is the profile containing credentials for GrafanaAssumeRole auth type in the shared credentials file
	ProfileName = "assume_role_credentials"
)

// Session factory.
// Stubbable by tests.
//
//nolint:gocritic
var newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
	s, err := session.NewSession(cfgs...)
	if err != nil {
		return nil, errorsource.DownstreamError(err, false)
	}
	return s, nil
}

// STS credentials factory.
// Stubbable by tests.
//
//nolint:gocritic
var newSTSCredentials = stscreds.NewCredentials

// EC2Metadata service factory.
// Stubbable by tests.
//
//nolint:gocritic
var newEC2Metadata = ec2metadata.New

// EC2 + ECS role credentials factory.
// Stubbable by tests.
var newRemoteCredentials = func(sess *session.Session) *credentials.Credentials {
	return credentials.NewCredentials(defaults.RemoteCredProvider(*sess.Config, sess.Handlers))
}

type GetSessionConfig struct {
	Settings      AWSDatasourceSettings
	HTTPClient    *http.Client
	UserAgentName *string
}

type SessionConfig struct {
	Settings      AWSDatasourceSettings
	HTTPClient    *http.Client
	UserAgentName *string
	AuthSettings  *AuthSettings
}

func isOptInRegion(region string) bool {
	// Opt-in region from https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
	regions := map[string]bool{
		"af-south-1":     true,
		"ap-east-1":      true,
		"ap-east-2":      true,
		"ap-south-2":     true,
		"ap-southeast-3": true,
		"ap-southeast-4": true,
		"ap-southeast-5": true,
		"ap-southeast-7": true,
		"ca-west-1":      true,
		"eu-central-2":   true,
		"eu-south-1":     true,
		"eu-south-2":     true,
		"il-central-1":   true,
		"me-central-1":   true,
		"me-south-1":     true,
		"mx-central-1":   true,
		// The rest of regions will return false
	}
	return regions[region]
}

// Deprecated: use GetSessionWithAuthSettings instead
func (sc *SessionCache) GetSession(c SessionConfig) (*session.Session, error) {
	if c.Settings.Region == "" && c.Settings.DefaultRegion != "" {
		// DefaultRegion is deprecated, Region should be used instead
		c.Settings.Region = c.Settings.DefaultRegion
	}

	// If the datasource calling GetSession is getting the settings from the contexts, they'll pass
	// the values through AuthSettings. Otherwise, we need to get them from the env variables.
	if c.AuthSettings == nil {
		c.AuthSettings = ReadAuthSettingsFromEnvironmentVariables()
	}

	authTypeAllowed := false
	for _, provider := range c.AuthSettings.AllowedAuthProviders {
		if provider == c.Settings.AuthType.String() {
			authTypeAllowed = true
			break
		}
	}

	if !authTypeAllowed {
		// user error, but mark as downstream
		return nil, errorsource.DownstreamError(fmt.Errorf("attempting to use an auth type that is not allowed: %q", c.Settings.AuthType.String()), false)
	}

	if c.Settings.AssumeRoleARN != "" && !c.AuthSettings.AssumeRoleEnabled {
		// user error, but mark as downstream
		return nil, errorsource.DownstreamError(fmt.Errorf("attempting to use assume role (ARN) which is disabled in grafana.ini"), false)
	}

	// Hash the settings to use as a cache key
	b := strings.Builder{}
	for i, s := range []string{
		c.Settings.AuthType.String(), c.Settings.AccessKey, c.Settings.SecretKey, c.Settings.Profile, c.Settings.AssumeRoleARN, c.Settings.Region, c.Settings.Endpoint,
	} {
		if i != 0 {
			b.WriteString(":")
		}
		b.WriteString(strings.ReplaceAll(s, ":", `\:`))
	}

	hashedSettings := sha256.Sum256([]byte(b.String()))
	cacheKey := fmt.Sprintf("%v", hashedSettings)

	// Check if we have a valid session in the cache, if so return it
	sc.sessCacheLock.RLock()
	if env, ok := sc.sessCache[cacheKey]; ok {
		if env.expiration.After(time.Now().UTC()) {
			sc.sessCacheLock.RUnlock()
			return env.session, nil
		}
	}
	sc.sessCacheLock.RUnlock()

	cfgs := []*aws.Config{
		{
			CredentialsChainVerboseErrors: aws.Bool(true),
			HTTPClient:                    c.HTTPClient,
		},
	}

	var regionCfg *aws.Config
	if c.Settings.Region == defaultRegion {
		backend.Logger.Warn("Region is set to \"default\", which is unsupported")
		c.Settings.Region = ""
	}
	if c.Settings.Region != "" {
		if c.Settings.AssumeRoleARN != "" && c.AuthSettings.AssumeRoleEnabled && isOptInRegion(c.Settings.Region) {
			// When assuming a role, the real region is set later in a new session
			// so we use a well-known region here (not opt-in) to obtain valid credentials
			regionCfg = &aws.Config{Region: aws.String("us-east-1")}

			// set regional endpoint flag to obtain credentials that can be used in opt-in regions as well
			optInRegionCfg := &aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint}

			cfgs = append(cfgs, regionCfg, optInRegionCfg)
		} else {
			regionCfg = &aws.Config{Region: aws.String(c.Settings.Region)}
			cfgs = append(cfgs, regionCfg)
		}
	}

	switch c.Settings.AuthType {
	case AuthTypeSharedCreds:
		backend.Logger.Debug("Authenticating towards AWS with shared credentials", "profile", c.Settings.Profile,
			"region", c.Settings.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewSharedCredentials(CredentialsPath, c.Settings.Profile),
		})
	case AuthTypeKeys:
		backend.Logger.Debug("Authenticating towards AWS with an access key pair", "region", c.Settings.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewStaticCredentials(c.Settings.AccessKey, c.Settings.SecretKey, c.Settings.SessionToken),
		})
	case AuthTypeDefault:
		backend.Logger.Debug("Authenticating towards AWS with default SDK method", "region", c.Settings.Region)
	case AuthTypeEC2IAMRole:
		backend.Logger.Debug("Authenticating towards AWS with IAM Role", "region", c.Settings.Region)
		sess, err := newSession(cfgs...)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, &aws.Config{Credentials: newRemoteCredentials(sess)})
	case AuthTypeGrafanaAssumeRole:
		backend.Logger.Debug("Authenticating towards AWS with Grafana Assume Role", "region", c.Settings.Region)
		accessKey, keyErr := os.ReadFile(awsTempCredsAccessKey)
		secretKey, secretErr := os.ReadFile(awsTempCredsSecretKey)
		if keyErr == nil && secretErr == nil {
			cfgs = append(cfgs, &aws.Config{
				Credentials: credentials.NewStaticCredentials(string(accessKey), string(secretKey), ""),
			})
			// if we don't find the files assume it's running single tenant and use the credentials file
		} else {
			cfgs = append(cfgs, &aws.Config{
				Credentials: credentials.NewSharedCredentials(CredentialsPath, ProfileName),
			})
		}
	default:
		return nil, fmt.Errorf("unrecognized authType: %d", c.Settings.AuthType)
	}

	duration := stscreds.DefaultDuration
	if c.AuthSettings.SessionDuration != nil {
		duration = *c.AuthSettings.SessionDuration
	}
	expiration := time.Now().UTC().Add(duration)

	if c.Settings.Endpoint != "" {
		cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(c.Settings.Endpoint)})
	}

	if c.Settings.AssumeRoleARN != "" && c.AuthSettings.AssumeRoleEnabled {
		// We should assume a role in AWS
		backend.Logger.Debug("Trying to assume role in AWS", "arn", c.Settings.AssumeRoleARN)

		// If a FIPS endpoint is set, we need to use the FIPS STS endpoint
		if c.Settings.Endpoint != "" {
			var endpoint = aws.String(getSTSEndpoint(c.Settings.Endpoint))
			cfgs = append(cfgs, &aws.Config{Endpoint: endpoint})
		}

		sess, err := newSession(cfgs...)
		if err != nil {
			return nil, err
		}

		cfgs = []*aws.Config{
			{
				CredentialsChainVerboseErrors: aws.Bool(true),
			},
			{
				// The previous session is used to obtain STS Credentials
				Credentials: newSTSCredentials(sess, c.Settings.AssumeRoleARN, func(p *stscreds.AssumeRoleProvider) {
					// Not sure if this is necessary, overlaps with p.Duration and is undocumented
					p.SetExpiration(expiration, 0)
					p.Duration = duration
					if c.Settings.AuthType == AuthTypeGrafanaAssumeRole {
						p.ExternalID = aws.String(c.AuthSettings.ExternalID)
					} else if c.Settings.ExternalID != "" {
						p.ExternalID = aws.String(c.Settings.ExternalID)
					}
				}),
			},
		}

		if c.Settings.Region != "" {
			regionCfg = &aws.Config{Region: aws.String(c.Settings.Region)}
			cfgs = append(cfgs, regionCfg)
		}

		// If a FIPS endpoint is set, we need to set the endpoint on the returned session
		if isFIPSEndpoint(c.Settings.Endpoint) {
			cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(c.Settings.Endpoint)})
		}
	}

	sess, err := newSession(cfgs...)
	if err != nil {
		return nil, err
	}

	if c.UserAgentName != nil {
		sess.Handlers.Send.PushFront(func(r *request.Request) {
			r.HTTPRequest.Header.Set("User-Agent", GetUserAgentString(*c.UserAgentName))
		})
	}

	backend.Logger.Debug("Successfully created AWS session")

	sc.sessCacheLock.Lock()
	sc.sessCache[cacheKey] = envelope{
		session:    sess,
		expiration: expiration,
	}
	sc.sessCacheLock.Unlock()

	return sess, nil
}

// AuthSettings can be grabed from the datasource instance's context with ReadAuthSettingsFromContext
func (sc *SessionCache) GetSessionWithAuthSettings(c GetSessionConfig, as AuthSettings) (*session.Session, error) {
	return sc.GetSession(SessionConfig{
		Settings:      c.Settings,
		HTTPClient:    c.HTTPClient,
		UserAgentName: c.UserAgentName,
		AuthSettings:  &as,
	})
}

// getSTSEndpoint returns true if the set endpoint is a fips endpoint
func isFIPSEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "fips") ||
		strings.Contains(endpoint, "us-gov-east-1") ||
		strings.Contains(endpoint, "us-gov-west-1")
}

// getSTSEndpoint checks if the set endpoint is a fips endpoint, and if so, returns the STS fips endpoint for the same region
func getSTSEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, "fips") {
		switch {
		case strings.Contains(endpoint, "us-east-1"):
			return "sts-fips.us-east-1.amazonaws.com"
		case strings.Contains(endpoint, "us-east-2"):
			return "sts-fips.us-east-2.amazonaws.com"
		case strings.Contains(endpoint, "us-west-1"):
			return "sts-fips.us-west-1.amazonaws.com"
		case strings.Contains(endpoint, "us-west-2"):
			return "sts-fips.us-west-2.amazonaws.com"
		}
	}

	if strings.Contains(endpoint, "us-gov-east-1") {
		return "sts.us-gov-east-1.amazonaws.com"
	}
	if strings.Contains(endpoint, "us-gov-west-1") {
		return "sts.us-gov-west-1.amazonaws.com"
	}
	return endpoint
}

// CredentialsProviderV2 provides a CredentialsProvider suitable for use with aws-sdk-go-v2,
// to be used while migrating datasources.
// Experimental: this works but is not thoroughly tested yet
func (sc *SessionCache) CredentialsProviderV2(ctx context.Context, cfg GetSessionConfig) (awsV2.CredentialsProvider, error) {
	authSettings := ReadAuthSettings(ctx)
	sess, err := sc.GetSessionWithAuthSettings(cfg, *authSettings)
	if err != nil {
		return nil, err
	}
	return &SessionCredentialsProvider{sess}, nil

}

type SessionCredentialsProvider struct {
	session *session.Session
}

func (scp *SessionCredentialsProvider) Retrieve(_ context.Context) (awsV2.Credentials, error) {
	creds := awsV2.Credentials{}
	v1creds, err := scp.session.Config.Credentials.Get()
	if err != nil {
		return creds, err
	}
	creds.AccessKeyID = v1creds.AccessKeyID
	creds.SecretAccessKey = v1creds.SecretAccessKey
	creds.SessionToken = v1creds.SessionToken
	return creds, nil
}
