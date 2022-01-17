package awsds

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

var plog = backend.Logger

type envelope struct {
	session    *session.Session
	expiration time.Time
}

// SessionCache cache sessions for a while
type SessionCache struct {
	sessCache     map[string]envelope
	sessCacheLock sync.RWMutex
	authSettings  *AuthSettings
}

// NewSessionCache creates a new session cache using the default settings loaded from environment variables
func NewSessionCache() *SessionCache {
	return &SessionCache{
		sessCache:    map[string]envelope{},
		authSettings: ReadAuthSettingsFromEnvironmentVariables(),
	}
}

// AllowedAuthProvidersEnvVarKeyName is the string literal for the aws allowed auth providers environment variable key name
const AllowedAuthProvidersEnvVarKeyName = "AWS_AUTH_AllowedAuthProviders"

// AssumeRoleEnabledEnvVarKeyName is the string literal for the aws assume role enabled environment variable key name
const AssumeRoleEnabledEnvVarKeyName = "AWS_AUTH_AssumeRoleEnabled"

func ReadAuthSettingsFromEnvironmentVariables() *AuthSettings {
	allowedAuthProviders := []string{}
	providers := os.Getenv(AllowedAuthProvidersEnvVarKeyName)
	for _, authProvider := range strings.Split(providers, ",") {
		authProvider = strings.TrimSpace(authProvider)
		if authProvider != "" {
			allowedAuthProviders = append(allowedAuthProviders, authProvider)
		}
	}

	if len(allowedAuthProviders) == 0 {
		allowedAuthProviders = []string{"default", "keys", "credentials"}
		plog.Warn("could not find allowed auth providers. falling back to 'default, keys, credentials'")
	}

	assumeRoleEnabledString := os.Getenv(AssumeRoleEnabledEnvVarKeyName)
	if len(assumeRoleEnabledString) == 0 {
		plog.Warn("environment variable '%s' missing. falling back to enable assume role", AssumeRoleEnabledEnvVarKeyName)
		assumeRoleEnabledString = "true"
	}

	assumeRoleEnabled, err := strconv.ParseBool(assumeRoleEnabledString)
	if err != nil {
		plog.Error("could not parse env variable '%s'", AssumeRoleEnabledEnvVarKeyName)
		assumeRoleEnabled = true
	}

	return &AuthSettings{
		AllowedAuthProviders: allowedAuthProviders,
		AssumeRoleEnabled:    assumeRoleEnabled,
	}
}

// Session factory.
// Stubbable by tests.
//nolint:gocritic
var newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
	return session.NewSession(cfgs...)
}

// STS credentials factory.
// Stubbable by tests.
//nolint:gocritic
var newSTSCredentials = stscreds.NewCredentials

// EC2Metadata service factory.
// Stubbable by tests.
//nolint:gocritic
var newEC2Metadata = ec2metadata.New

// EC2 role credentials factory.
// Stubbable by tests.
var newEC2RoleCredentials = func(sess *session.Session) *credentials.Credentials {
	return credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(sess), ExpiryWindow: stscreds.DefaultDuration})
}

type SessionConfig struct {
	Settings      AWSDatasourceSettings
	HTTPClient    *http.Client
	UserAgentName *string
}

// GetSession returns a session from the config and possible region overrides -- implements AmazonSessionProvider
func (sc *SessionCache) GetSession(c SessionConfig) (*session.Session, error) {
	if c.Settings.Region == "" && c.Settings.DefaultRegion != "" {
		// DefaultRegion is deprecated, Region should be used instead
		c.Settings.Region = c.Settings.DefaultRegion
	}
	authTypeAllowed := false
	for _, provider := range sc.authSettings.AllowedAuthProviders {
		if provider == c.Settings.AuthType.String() {
			authTypeAllowed = true
			break
		}
	}
	if !authTypeAllowed {
		return nil, fmt.Errorf("attempting to use an auth type that is not allowed: %q", c.Settings.AuthType.String())
	}

	if c.Settings.AssumeRoleARN != "" && !sc.authSettings.AssumeRoleEnabled {
		return nil, fmt.Errorf("attempting to use assume role (ARN) which is disabled in grafana.ini")
	}

	bldr := strings.Builder{}
	for i, s := range []string{
		c.Settings.AuthType.String(), c.Settings.AccessKey, c.Settings.Profile, c.Settings.AssumeRoleARN, c.Settings.Region, c.Settings.Endpoint,
	} {
		if i != 0 {
			bldr.WriteString(":")
		}
		bldr.WriteString(strings.ReplaceAll(s, ":", `\:`))
	}
	cacheKey := bldr.String()

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
		plog.Warn("Region is set to \"default\", which is unsupported")
		c.Settings.Region = ""
	}
	if c.Settings.Region != "" {
		regionCfg = &aws.Config{Region: aws.String(c.Settings.Region)}
		cfgs = append(cfgs, regionCfg)
	}

	if c.Settings.Endpoint != "" {
		cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(c.Settings.Endpoint)})
	}

	switch c.Settings.AuthType {
	case AuthTypeSharedCreds:
		plog.Debug("Authenticating towards AWS with shared credentials", "profile", c.Settings.Profile,
			"region", c.Settings.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewSharedCredentials("", c.Settings.Profile),
		})
	case AuthTypeKeys:
		plog.Debug("Authenticating towards AWS with an access key pair", "region", c.Settings.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewStaticCredentials(c.Settings.AccessKey, c.Settings.SecretKey, c.Settings.SessionToken),
		})
	case AuthTypeDefault:
		plog.Debug("Authenticating towards AWS with default SDK method", "region", c.Settings.Region)
	case AuthTypeEC2IAMRole:
		plog.Debug("Authenticating towards AWS with IAM Role", "region", c.Settings.Region)
		sess, err := newSession(cfgs...)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, &aws.Config{Credentials: newEC2RoleCredentials(sess)})
	default:
		panic(fmt.Sprintf("Unrecognized authType: %d", c.Settings.AuthType))
	}
	sess, err := newSession(cfgs...)
	if err != nil {
		return nil, err
	}

	duration := stscreds.DefaultDuration
	expiration := time.Now().UTC().Add(duration)
	if c.Settings.AssumeRoleARN != "" && sc.authSettings.AssumeRoleEnabled {
		// We should assume a role in AWS
		plog.Debug("Trying to assume role in AWS", "arn", c.Settings.AssumeRoleARN)

		cfgs := []*aws.Config{
			{
				CredentialsChainVerboseErrors: aws.Bool(true),
			},
			{
				Credentials: newSTSCredentials(sess, c.Settings.AssumeRoleARN, func(p *stscreds.AssumeRoleProvider) {
					// Not sure if this is necessary, overlaps with p.Duration and is undocumented
					p.Expiry.SetExpiration(expiration, 0)
					p.Duration = duration
					if c.Settings.ExternalID != "" {
						p.ExternalID = aws.String(c.Settings.ExternalID)
					}
				}),
			},
		}
		if regionCfg != nil {
			cfgs = append(cfgs, regionCfg)
		}
		sess, err = newSession(cfgs...)
		if err != nil {
			return nil, err
		}
	}

	if c.UserAgentName != nil {
		sess.Handlers.Send.PushFront(func(r *request.Request) {
			r.HTTPRequest.Header.Set("User-Agent", GetUserAgentString(*c.UserAgentName))
		})
	}

	plog.Debug("Successfully created AWS session")

	sc.sessCacheLock.Lock()
	sc.sessCache[cacheKey] = envelope{
		session:    sess,
		expiration: expiration,
	}
	sc.sessCacheLock.Unlock()

	return sess, nil
}
