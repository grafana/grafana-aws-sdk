package awsds

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grafana/grafana-aws-sdk/pkg/auth"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type envelope struct {
	session    *session.Session
	expiration time.Time
}

// SessionCache cache sessions for a while
type SessionCache struct {
	sessCache     map[string]envelope
	sessCacheLock sync.RWMutex
	authSettings  *auth.AuthSettings
}

// NewSessionCache creates a new session cache using the default settings loaded from environment variables
func NewSessionCache() *SessionCache {
	return &SessionCache{
		sessCache:    map[string]envelope{},
		authSettings: auth.ReadAuthSettingsFromEnvironmentVariables(),
	}
}

// Session factory.
// Stubbable by tests.
//
//nolint:gocritic
var newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
	return session.NewSession(cfgs...)
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

type SessionConfig struct {
	Settings      AWSDatasourceSettings
	HTTPClient    *http.Client
	UserAgentName *string
}

func isOptInRegion(region string) bool {
	// Opt-in region from https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
	regions := map[string]bool{
		"af-south-1":     true,
		"ap-east-1":      true,
		"ap-south-2":     true,
		"ap-southeast-3": true,
		"ap-southeast-4": true,
		"eu-central-2":   true,
		"eu-south-1":     true,
		"eu-south-2":     true,
		"il-central-1":   true,
		"me-central-1":   true,
		"me-south-1":     true,
		// The rest of regions will return false
	}
	return regions[region]
}

// GetSession returns a session from the config and possible region overrides -- implements AmazonSessionProvider
func (sc *SessionCache) GetSession(c SessionConfig) (*session.Session, error) {
	// set Region to the default region
	if c.Settings.Region == "" && c.Settings.DefaultRegion != "" {
		// DefaultRegion is deprecated, Region should be used instead
		c.Settings.Region = c.Settings.DefaultRegion
	}

	// if the region is literally the world "default" unset it
	if c.Settings.Region == defaultRegion {
		backend.Logger.Warn("Region is set to \"default\", which is unsupported")
		c.Settings.Region = ""
	}

	// Hash the settings to use as a cache key
	bldr := strings.Builder{}
	for i, s := range []string{
		c.Settings.AuthType.String(), c.Settings.AccessKey, c.Settings.SecretKey, c.Settings.Profile, c.Settings.AssumeRoleARN, c.Settings.Region, c.Settings.Endpoint,
	} {
		if i != 0 {
			bldr.WriteString(":")
		}
		bldr.WriteString(strings.ReplaceAll(s, ":", `\:`))
	}

	hashedSettings := sha256.Sum256([]byte(bldr.String()))
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

	if c.Settings.Region != "" {
		if c.Settings.AssumeRoleARN != "" && sc.authSettings.AssumeRoleEnabled && isOptInRegion(c.Settings.Region) {
			// When assuming a role, the real region is set later in a new session
			// so we use a well-known region here (not opt-in) to obtain valid credentials
			regionCfg = &aws.Config{Region: aws.String("us-east-1")}
			cfgs = append(cfgs, regionCfg)
		} else {
			regionCfg = &aws.Config{Region: aws.String(c.Settings.Region)}
			cfgs = append(cfgs, regionCfg)
		}
	}

	creds, err := auth.GetCredsFromConfig(auth.CredentialsConfig{
		AuthType: c.Settings.AuthType.String(),
	})
	if err != nil {
		return nil, err
	}
	cfgs = append(cfgs, &aws.Config{
		Credentials: creds,
	})

	if c.Settings.Endpoint != "" {
		cfgs = append(cfgs, &aws.Config{Endpoint: aws.String(c.Settings.Endpoint)})
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
	duration := stscreds.DefaultDuration
	if sc.authSettings.SessionDuration != nil {
		duration = *sc.authSettings.SessionDuration
	}
	expiration := time.Now().UTC().Add(duration)
	sc.sessCache[cacheKey] = envelope{
		session:    sess,
		expiration: expiration,
	}
	sc.sessCacheLock.Unlock()

	return sess, nil
}
