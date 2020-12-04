package awsds

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
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
}

// NewSessionCache creates a new session cache
func NewSessionCache() SessionCache {
	return SessionCache{
		sessCache: map[string]envelope{},
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

// GetSession returns a session from the config and possible region overrides -- implements AmazonSessionProvider
func (sc *SessionCache) GetSession(region string, s AWSDatasourceSettings) (*session.Session, error) {
	if region == "" || region == defaultRegion {
		region = s.Region
	}

	bldr := strings.Builder{}
	for i, s := range []string{
		s.AuthType.String(), s.AccessKey, s.Profile, s.AssumeRoleARN, region,
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
	if region == defaultRegion {
		plog.Warn("Region is set to \"default\", which is unsupported")
		region = ""
	}
	if region != "" {
		regionCfg = &aws.Config{Region: aws.String(region)}
		cfgs = append(cfgs, regionCfg)
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
	default:
		panic(fmt.Sprintf("Unrecognized authType: %d", s.AuthType))
	}
	sess, err := newSession(cfgs...)
	if err != nil {
		return nil, err
	}

	duration := stscreds.DefaultDuration
	expiration := time.Now().UTC().Add(duration)
	if s.AssumeRoleARN != "" {
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
