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
		resetEnvironmentVariables()
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
		resetEnvironmentVariables()
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

	t.Run("Assume role not enabled", func(t *testing.T) {
		resetEnvironmentVariables()
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
		resetEnvironmentVariables()
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
}

func TestNewSession_AllowedAuthProviders(t *testing.T) {
	t.Run("Not allowed auth type is used", func(t *testing.T) {
		resetEnvironmentVariables()
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
		resetEnvironmentVariables()
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
			resetEnvironmentVariables()
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
	newEC2RoleCredentials = func(sess *session.Session) *credentials.Credentials {
		return credentials.NewCredentials(&ec2rolecreds.EC2RoleProvider{Client: newEC2Metadata(nil), ExpiryWindow: stscreds.DefaultDuration})
	}

	t.Run("Credentials are created", func(t *testing.T) {
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeEC2IAMRole,
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
	})
}

func resetEnvironmentVariables() {
	os.Unsetenv(AllowedAuthProvidersEnvVarKeyName)
	os.Unsetenv(AssumeRoleEnabledEnvVarKeyName)
}

func TestWithUserAgent(t *testing.T) {
	resetEnvironmentVariables()
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
	resetEnvironmentVariables()
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
