package awsds

import (
	"fmt"
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
	authSettings := readAuthSettingsFromEnvironmentVariables()
	return &NewSessionCacheWithSettigns(authSettings)
}

// NewSessionCacheWithSettigns creates a new session cache using explict settings
func NewSessionCacheWithSettigns(s *AuthSettings) SessionCache {
	return SessionCache{
		sessCache:    map[string]envelope{},
		authSettings: s,
	}
}

const ENV_VAR_AllowedAuthProviders = "AWS_AUTH_AllowedAuthProviders"
const ENV_VAR_AssumeRoleEnabled = "AWS_AUTH_AssumeRoleEnabled"

func readAuthSettingsFromEnvironmentVariables() *AuthSettings {
	allowedAuthProviders := []string{}
	providers := os.Getenv(ENV_VAR_AllowedAuthProviders)
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

	assumeRoleEnabled, err := strconv.ParseBool(os.Getenv(ENV_VAR_AssumeRoleEnabled))
	if err != nil {
		plog.Warn("could not parse env variable '%s'", ENV_VAR_AssumeRoleEnabled)
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

// GetSession returns a session from the config and possible region overrides -- implements AmazonSessionProvider
func (sc *SessionCache) GetSession(region string, s AWSDatasourceSettings) (*session.Session, error) {
	if region == "" || region == defaultRegion {
		region = s.Region
	}

	authTypeAllowed := false
	for _, provider := range sc.authSettings.AllowedAuthProviders {
		if provider == s.AuthType.String() {
			authTypeAllowed = true
			break
		}
	}
	if !authTypeAllowed {
		return nil, fmt.Errorf("attempting to use an auth type that is not allowed: %q", s.AuthType.String())
	}

	if s.AssumeRoleARN != "" && !sc.authSettings.AssumeRoleEnabled {
		return nil, fmt.Errorf("attempting to use assume role (ARN) which is disabled in grafana.ini")
	}

	bldr := strings.Builder{}
	for i, s := range []string{
		s.AuthType.String(), s.AccessKey, s.Profile, s.AssumeRoleARN, region, s.Endpoint,
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
		},
	}

	var regionCfg *aws.Config
	if s.Region == defaultRegion {
		plog.Warn("Region is set to \"default\", which is unsupported")
		s.Region = ""
	}
	if s.Region != "" {
		regionCfg = &aws.Config{Region: aws.String(s.Region)}
		cfgs = append(cfgs, regionCfg)
	}

	if s.Endpoint != "" {
		cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(s.Endpoint)})
	}

	switch s.AuthType {
	case AuthTypeSharedCreds:
		plog.Debug("Authenticating towards AWS with shared credentials", "profile", s.Profile,
			"region", s.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewSharedCredentials("", s.Profile),
		})
	case AuthTypeKeys:
		plog.Debug("Authenticating towards AWS with an access key pair", "region", s.Region)
		cfgs = append(cfgs, &aws.Config{
			Credentials: credentials.NewStaticCredentials(s.AccessKey, s.SecretKey, ""),
		})
	case AuthTypeDefault:
		plog.Debug("Authenticating towards AWS with default SDK method", "region", s.Region)
	case AuthTypeEC2IAMRole:
		plog.Debug("Authenticating towards AWS with IAM Role", "region", s.Region)
		sess, err := newSession(cfgs...)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, &aws.Config{Credentials: newEC2RoleCredentials(sess)})
	default:
		panic(fmt.Sprintf("Unrecognized authType: %d", s.AuthType))
	}
	sess, err := newSession(cfgs...)
	if err != nil {
		return nil, err
	}

	duration := stscreds.DefaultDuration
	expiration := time.Now().UTC().Add(duration)
	if s.AssumeRoleARN != "" && sc.authSettings.AssumeRoleEnabled {
		// We should assume a role in AWS
		plog.Debug("Trying to assume role in AWS", "arn", s.AssumeRoleARN)

		cfgs := []*aws.Config{
			{
				CredentialsChainVerboseErrors: aws.Bool(true),
			},
			{
				Credentials: newSTSCredentials(sess, s.AssumeRoleARN, func(p *stscreds.AssumeRoleProvider) {
					// Not sure if this is necessary, overlaps with p.Duration and is undocumented
					p.Expiry.SetExpiration(expiration, 0)
					p.Duration = duration
					if s.ExternalID != "" {
						p.ExternalID = aws.String(s.ExternalID)
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

	plog.Debug("Successfully created AWS session")

	sc.sessCacheLock.Lock()
	sc.sessCache[cacheKey] = envelope{
		session:    sess,
		expiration: expiration,
	}
	sc.sessCacheLock.Unlock()

	return sess, nil
}
