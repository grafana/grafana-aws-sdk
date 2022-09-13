package awsds

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test cloudWatchExecutor.newSession with assumption of IAM role.
func TestNewSession_AssumeRole(t *testing.T) {
	origNewSession := newSession
	origNewSTSCredentials := newSTSCredentials
	origNewEC2Metadata := newEC2Metadata
	t.Cleanup(func() {
		newSession = origNewSession
		newSTSCredentials = origNewSTSCredentials
		newEC2Metadata = origNewEC2Metadata
	})
	newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
		cfg := aws.Config{}
		cfg.MergeIn(cfgs...)
		return &session.Session{
			Config: &cfg,
		}, nil
	}
	newSTSCredentials = func(c client.ConfigProvider, roleARN string,
		options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
		p := &stscreds.AssumeRoleProvider{
			RoleARN: roleARN,
		}
		for _, o := range options {
			o(p)
		}

		return credentials.NewCredentials(p)
	}
	newEC2Metadata = func(p client.ConfigProvider, cfgs ...*aws.Config) *ec2metadata.EC2Metadata {
		return nil
	}
	duration := stscreds.DefaultDuration

	t.Run("Without external ID", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.NoError(t, err)
		require.NotNil(t, sess)
		expCreds := credentials.NewCredentials(&stscreds.AssumeRoleProvider{
			RoleARN:  roleARN,
			Duration: duration,
		})
		diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
			return true
		}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
		assert.Empty(t, diff)
	})

	t.Run("With external ID", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		const roleARN = "test"
		const externalID = "external"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
			ExternalID:    externalID,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.NoError(t, err)
		require.NotNil(t, sess)
		expCreds := credentials.NewCredentials(&stscreds.AssumeRoleProvider{
			RoleARN:    roleARN,
			ExternalID: aws.String(externalID),
			Duration:   duration,
		})
		diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
			return true
		}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
		assert.Empty(t, diff)
	})

	t.Run("With custom duration", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true")
		os.Setenv(SessionDurationEnvVarKeyName, "20m")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.NoError(t, err)
		require.NotNil(t, sess)
		expCreds := credentials.NewCredentials(&stscreds.AssumeRoleProvider{
			RoleARN:  roleARN,
			Duration: 1200000000000, //20 minutes in nanoseconds count
		})
		diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
			return true
		}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
		assert.Empty(t, diff)
	})

	t.Run("Assume role not enabled", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "false")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.Error(t, err)
		require.Nil(t, sess)
		expectedError := "attempting to use assume role (ARN) which is disabled in grafana.ini"
		assert.Equal(t, expectedError, err.Error())
	})

	t.Run("Assume role is enabled when AssumeRoleEnabled env var is missing", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.NoError(t, err)
		require.NotNil(t, sess)
		expCreds := credentials.NewCredentials(&stscreds.AssumeRoleProvider{
			RoleARN:  roleARN,
			Duration: duration,
		})
		diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
			return true
		}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
		assert.Empty(t, diff)
	})

	t.Run("Assume role is enabled with an opt-in region", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		fakeNewSTSCredentials := newSTSCredentials
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			sess := c.(*session.Session)
			// Verify that we are using the well-known region
			assert.Equal(t, "us-east-1", *sess.Config.Region)
			return fakeNewSTSCredentials(c, roleARN, options...)
		}
		settings := AWSDatasourceSettings{
			AssumeRoleARN: "test",
			Region:        "me-south-1",
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		newSTSCredentials = fakeNewSTSCredentials

		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.Equal(t, "me-south-1", *sess.Config.Region)
	})

	t.Run("Assume role is enabled with a gov region", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		fakeNewSTSCredentials := newSTSCredentials
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			sess := c.(*session.Session)
			// Verify that we are using the well-known region
			assert.Equal(t, "us-gov-east-1", *sess.Config.Region)
			return fakeNewSTSCredentials(c, roleARN, options...)
		}
		settings := AWSDatasourceSettings{
			AssumeRoleARN: "test",
			Region:        "us-gov-east-1",
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		newSTSCredentials = fakeNewSTSCredentials

		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.Equal(t, "us-gov-east-1", *sess.Config.Region)
	})
}

func TestNewSession_AllowedAuthProviders(t *testing.T) {
	t.Run("Not allowed auth type is used", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeDefault,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "keys")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.Error(t, err)
		require.Nil(t, sess)
		assert.Equal(t, `attempting to use an auth type that is not allowed: "default"`, err.Error())
	})

	t.Run("Allowed auth type is used", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeKeys,
		}
		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "keys")
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.NoError(t, err)
		require.NotNil(t, sess)
	})

	t.Run("Fallback is used when AllowedAuthProviders env var is missing", func(t *testing.T) {
		defaultAuthProviders := []AuthType{AuthTypeDefault, AuthTypeKeys, AuthTypeSharedCreds}
		for _, provider := range defaultAuthProviders {
			defer unsetEnvironmentVariables()
			settings := AWSDatasourceSettings{
				AuthType: provider,
			}
			cache := NewSessionCache()
			sess, err := cache.GetSession(SessionConfig{Settings: settings})
			require.NoError(t, err)
			require.NotNil(t, sess)
		}
	})
}

func TestNewSession_EC2IAMRole(t *testing.T) {
	newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
		cfg := aws.Config{}
		cfg.MergeIn(cfgs...)
		return &session.Session{
			Config: &cfg,
		}, nil
	}
	newEC2Metadata = func(p client.ConfigProvider, cfgs ...*aws.Config) *ec2metadata.EC2Metadata {
		return nil
	}
	newRemoteCredentials = func(sess *session.Session) *credentials.Credentials {
		return credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{Client: newEC2Metadata(nil), ExpiryWindow: stscreds.DefaultDuration})
	}

	t.Run("Credentials are created", func(t *testing.T) {
		credentialCfgs := []*aws.Config{}
		newSession = func(cfgs ...*aws.Config) (*session.Session, error) {
			cfg := aws.Config{}
			cfg.MergeIn(cfgs...)
			credentialCfgs = append(credentialCfgs, &cfg)
			return &session.Session{
				Config: &cfg,
			}, nil
		}
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeEC2IAMRole,
			Endpoint: "foo",
		}

		os.Setenv(AllowedAuthProvidersEnvVarKeyName, "ec2_iam_role")
		os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true")

		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings})
		require.NoError(t, err)
		require.NotNil(t, sess)

		expCreds := credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{
			Client: newEC2Metadata(nil), ExpiryWindow: stscreds.DefaultDuration,
		})
		diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
			return true
		}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
		assert.Empty(t, diff)

		// Endpoint should be added to final session but not configuration session
		require.Equal(t, 2, len(credentialCfgs))
		require.Nil(t, credentialCfgs[0].Endpoint)
		require.NotNil(t, credentialCfgs[1].Endpoint)
		require.NotNil(t, sess.Config.Endpoint)
		require.Equal(t, "foo", *sess.Config.Endpoint)
	})
}

func unsetEnvironmentVariables() {
	os.Unsetenv(AllowedAuthProvidersEnvVarKeyName)
	os.Unsetenv(AssumeRoleEnabledEnvVarKeyName)
	os.Unsetenv(SessionDurationEnvVarKeyName)
}

func TestWithUserAgent(t *testing.T) {
	defer unsetEnvironmentVariables()
	os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
	os.Setenv(AssumeRoleEnabledEnvVarKeyName, "false")
	cache := NewSessionCache()
	sess, err := cache.GetSession(SessionConfig{UserAgentName: aws.String("Athena")})
	require.NoError(t, err)
	require.NotNil(t, sess)
	req := &request.Request{
		HTTPRequest: httptest.NewRequest(http.MethodGet, "/upper?word=abc", nil),
	}
	sess.Handlers.Send.Run(req)

	res := req.HTTPRequest.Header.Get("User-Agent")
	assert.Contains(t, res, "Athena/dev")
}

func TestWithCustomHTTPClient(t *testing.T) {
	defer unsetEnvironmentVariables()
	os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default")
	os.Setenv(AssumeRoleEnabledEnvVarKeyName, "false")
	cache := NewSessionCache()
	sess, err := cache.GetSession(SessionConfig{
		HTTPClient: &http.Client{Timeout: 123},
	})
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Equal(t, time.Duration(123), sess.Config.HTTPClient.Timeout)
}
