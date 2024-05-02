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
	"github.com/aws/aws-sdk-go/aws/endpoints"
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
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"default"},
				AssumeRoleEnabled:    true,
			},
		})
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
		const roleARN = "test"
		const externalID = "external"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
			ExternalID:    externalID,
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"default"},
				AssumeRoleEnabled:    true,
			},
		})
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
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		expectedDuration := 20 * time.Minute

		t.Run("from config", func(t *testing.T) {
			cache := NewSessionCache()
			sess, err := cache.GetSession(SessionConfig{
				Settings: settings,
				AuthSettings: &AuthSettings{
					AllowedAuthProviders: []string{"default"},
					AssumeRoleEnabled:    true,
					SessionDuration:      &expectedDuration,
				},
			})
			require.NoError(t, err)
			require.NotNil(t, sess)
			expCreds := credentials.NewCredentials(&stscreds.AssumeRoleProvider{
				RoleARN:  roleARN,
				Duration: expectedDuration,
			})
			diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
				return true
			}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
			assert.Empty(t, diff)
		})

		t.Run("from env variable", func(t *testing.T) {
			defer unsetEnvironmentVariables()
			require.NoError(t, os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default"))
			require.NoError(t, os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true"))
			require.NoError(t, os.Setenv(SessionDurationEnvVarKeyName, "20m"))
			cache := NewSessionCache()
			sess, err := cache.GetSession(SessionConfig{Settings: settings})
			require.NoError(t, err)
			require.NotNil(t, sess)
			expCreds := credentials.NewCredentials(&stscreds.AssumeRoleProvider{
				RoleARN:  roleARN,
				Duration: expectedDuration,
			})
			diff := cmp.Diff(expCreds, sess.Config.Credentials, cmp.Exporter(func(_ reflect.Type) bool {
				return true
			}), cmpopts.IgnoreFields(stscreds.AssumeRoleProvider{}, "Expiry"))
			assert.Empty(t, diff)
		})
	})

	t.Run("Assume role not enabled", func(t *testing.T) {
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}

		t.Run("from config", func(t *testing.T) {
			cache := NewSessionCache()
			sess, err := cache.GetSession(SessionConfig{
				Settings: settings,
				AuthSettings: &AuthSettings{
					AllowedAuthProviders: []string{"default"},
					AssumeRoleEnabled:    false,
				},
			})
			require.Error(t, err)
			require.Nil(t, sess)
			expectedError := "attempting to use assume role (ARN) which is disabled in grafana.ini"
			assert.Equal(t, expectedError, err.Error())
		})

		t.Run("from env variable", func(t *testing.T) {
			defer unsetEnvironmentVariables()
			require.NoError(t, os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default"))
			require.NoError(t, os.Setenv(AssumeRoleEnabledEnvVarKeyName, "false"))
			cache := NewSessionCache()
			sess, err := cache.GetSession(SessionConfig{Settings: settings})
			require.Error(t, err)
			require.Nil(t, sess)
			expectedError := "attempting to use assume role (ARN) which is disabled in grafana.ini"
			assert.Equal(t, expectedError, err.Error())
		})
	})

	t.Run("Assume role is enabled when AssumeRoleEnabled env var is missing and no config set", func(t *testing.T) {
		defer unsetEnvironmentVariables()
		const roleARN = "test"
		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		require.NoError(t, os.Setenv(AllowedAuthProvidersEnvVarKeyName, "default"))
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
		fakeNewSTSCredentials := newSTSCredentials
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			sess := c.(*session.Session)
			// Verify that we are using the well-known region
			assert.Equal(t, "us-east-1", *sess.Config.Region)
			// verify that we're using regional sts endpoint
			assert.Equal(t, endpoints.RegionalSTSEndpoint, sess.Config.STSRegionalEndpoint)
			return fakeNewSTSCredentials(c, roleARN, options...)
		}
		settings := AWSDatasourceSettings{
			AssumeRoleARN: "test",
			Region:        "me-south-1",
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"default"},
				AssumeRoleEnabled:    true,
			},
		})
		newSTSCredentials = fakeNewSTSCredentials

		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.Equal(t, "me-south-1", *sess.Config.Region)
	})

	t.Run("Assume role is enabled with a gov region", func(t *testing.T) {
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
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"default"},
				AssumeRoleEnabled:    true,
			},
		})
		newSTSCredentials = fakeNewSTSCredentials

		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.Equal(t, "us-gov-east-1", *sess.Config.Region)
	})

	t.Run("Assume role is enabled with a fips endpoint", func(t *testing.T) {
		fakeNewSTSCredentials := newSTSCredentials
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			sess := c.(*session.Session)
			// Verify that we are using the correct sts endpoint
			assert.Equal(t, "sts-fips.us-east-1.amazonaws.com", *sess.Config.Endpoint)
			return fakeNewSTSCredentials(c, roleARN, options...)
		}
		settings := AWSDatasourceSettings{
			AssumeRoleARN: "test",
			Region:        "us-east-1",
			Endpoint:      "athena-fips.us-east-1.amazonaws.com",
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"default"},
				AssumeRoleEnabled:    true,
			},
		})
		newSTSCredentials = fakeNewSTSCredentials

		require.NoError(t, err)
		require.NotNil(t, sess)
		// Verify that we use the corrected fips endpoint, not the one from the settings
		assert.Equal(t, "sts-fips.us-east-1.amazonaws.com", *sess.Config.Endpoint)
	})

	t.Run("Assume role is enabled with a non-fips endpoint", func(t *testing.T) {
		fakeNewSTSCredentials := newSTSCredentials
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			sess := c.(*session.Session)
			// Verify that we are using the correct sts endpoint
			assert.Equal(t, "sts.eu-west-2.amazonaws.com", *sess.Config.Endpoint)
			return fakeNewSTSCredentials(c, roleARN, options...)
		}
		settings := AWSDatasourceSettings{
			AssumeRoleARN: "test",
			Region:        "eu-west-2",
			Endpoint:      "sts.eu-west-2.amazonaws.com",
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"default"},
				AssumeRoleEnabled:    true,
			},
		})
		newSTSCredentials = fakeNewSTSCredentials

		require.NoError(t, err)
		require.NotNil(t, sess)
		// Verify that we use the endpoint from the settings
		assert.Equal(t, settings.Endpoint, *sess.Config.Endpoint)
	})
}

func TestNewSession_AllowedAuthProviders(t *testing.T) {
	t.Run("Not allowed auth type is used", func(t *testing.T) {
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeDefault,
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"key"},
			},
		})
		require.Error(t, err)
		require.Nil(t, sess)
		assert.Equal(t, `attempting to use an auth type that is not allowed: "default"`, err.Error())
	})

	t.Run("Allowed auth type is used", func(t *testing.T) {
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeKeys,
		}
		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{
			Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"keys"},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, sess)
	})

	t.Run("Fallback is used when env variable and auth settings are missing", func(t *testing.T) {
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

func TestNewSession_GrafanaAssumeRole(t *testing.T) {
	origNewSTSCredentials := newSTSCredentials
	t.Cleanup(func() {
		newSTSCredentials = origNewSTSCredentials
	})

	t.Run("externalID is passed to the session", func(t *testing.T) {
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			p := &stscreds.AssumeRoleProvider{
				RoleARN: roleARN,
			}
			for _, o := range options {
				o(p)
			}
			require.NotNil(t, p.ExternalID)

			assert.Equal(t, "pretendExternalId", *p.ExternalID)
			return credentials.NewCredentials(p)
		}

		t.Run("from config", func(t *testing.T) {
			cache := NewSessionCache()
			_, err := cache.GetSession(SessionConfig{
				Settings: AWSDatasourceSettings{
					AuthType:      AuthTypeGrafanaAssumeRole,
					AssumeRoleARN: "test_arn",
				},
				AuthSettings: &AuthSettings{
					AllowedAuthProviders: []string{"grafana_assume_role"},
					ExternalID:           "pretendExternalId",
					AssumeRoleEnabled:    true,
				},
			})
			require.NoError(t, err)
		})

		t.Run("from env variable", func(t *testing.T) {
			originalExternalId := os.Getenv(GrafanaAssumeRoleExternalIdKeyName)
			require.NoError(t, os.Setenv(GrafanaAssumeRoleExternalIdKeyName, "pretendExternalId"))
			t.Cleanup(func() {
				os.Setenv(GrafanaAssumeRoleExternalIdKeyName, originalExternalId)
				unsetEnvironmentVariables()
			})
			require.NoError(t, os.Setenv(AllowedAuthProvidersEnvVarKeyName, "grafana_assume_role"))
			require.NoError(t, os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true"))

			cache := NewSessionCache()
			_, err := cache.GetSession(SessionConfig{
				Settings: AWSDatasourceSettings{
					AuthType:      AuthTypeGrafanaAssumeRole,
					AssumeRoleARN: "test_arn",
				},
			})
			require.NoError(t, err)
		})
	})

	t.Run("roleARN is passed to the session", func(t *testing.T) {
		newSTSCredentials = func(c client.ConfigProvider, roleARN string,
			options ...func(*stscreds.AssumeRoleProvider)) *credentials.Credentials {
			p := &stscreds.AssumeRoleProvider{
				RoleARN: roleARN,
			}
			for _, o := range options {
				o(p)
			}
			require.Equal(t, "test_arn", roleARN)
			return credentials.NewCredentials(p)
		}

		cache := NewSessionCache()
		_, err := cache.GetSession(SessionConfig{
			Settings: AWSDatasourceSettings{
				AuthType:      AuthTypeGrafanaAssumeRole,
				AssumeRoleARN: "test_arn",
			},
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"grafana_assume_role"},
				AssumeRoleEnabled:    true,
			},
		})

		require.NoError(t, err)
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

		cache := NewSessionCache()
		sess, err := cache.GetSession(SessionConfig{Settings: settings,
			AuthSettings: &AuthSettings{
				AllowedAuthProviders: []string{"ec2_iam_role"},
				AssumeRoleEnabled:    true,
			}})
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

func TestWithUserAgent(t *testing.T) {
	cache := NewSessionCache()
	sess, err := cache.GetSession(SessionConfig{UserAgentName: aws.String("Athena"),
		AuthSettings: &AuthSettings{
			AllowedAuthProviders: []string{"default"},
			AssumeRoleEnabled:    false,
		},
	})
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
	cache := NewSessionCache()
	sess, err := cache.GetSession(SessionConfig{
		HTTPClient: &http.Client{Timeout: 123},
		AuthSettings: &AuthSettings{
			AllowedAuthProviders: []string{"default"},
			AssumeRoleEnabled:    false,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Equal(t, time.Duration(123), sess.Config.HTTPClient.Timeout)
}
