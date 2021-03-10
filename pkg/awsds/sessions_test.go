package awsds

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
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

		cache := NewSessionCache(&AuthSettings{AllowedAuthProviders: []string{"default"}, AssumeRoleEnabled: true})

		sess, err := cache.GetSession(defaultRegion, settings)
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

		cache := NewSessionCache(&AuthSettings{AllowedAuthProviders: []string{"default"}, AssumeRoleEnabled: true})

		sess, err := cache.GetSession(defaultRegion, settings)
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
		const roleARN = "test"

		settings := AWSDatasourceSettings{
			AssumeRoleARN: roleARN,
		}
		cache := NewSessionCache(&AuthSettings{AllowedAuthProviders: []string{"default"}, AssumeRoleEnabled: false})

		sess, err := cache.GetSession(defaultRegion, settings)

		require.Error(t, err)
		require.Nil(t, sess)

		expectedError := "attempting to use assume role (ARN) which is disabled in grafana.ini"
		assert.Equal(t, expectedError, err.Error())
	})
}

func TestNewSession_AllowedAuthProviders(t *testing.T) {
	t.Run("Not allowed auth type is used", func(t *testing.T) {
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeDefault,
		}
		cache := NewSessionCache(&AuthSettings{AllowedAuthProviders: []string{"keys"}})
		sess, err := cache.GetSession(defaultRegion, settings)
		require.Error(t, err)
		require.Nil(t, sess)
		assert.Equal(t, `attempting to use an auth type that is not allowed: "default"`, err.Error())
	})

	t.Run("Allowed auth type is used", func(t *testing.T) {
		settings := AWSDatasourceSettings{
			AuthType: AuthTypeKeys,
		}
		cache := NewSessionCache(&AuthSettings{AllowedAuthProviders: []string{"keys"}})
		sess, err := cache.GetSession(defaultRegion, settings)
		require.NoError(t, err)
		require.NotNil(t, sess)
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

		cache := NewSessionCache(&AuthSettings{AllowedAuthProviders: []string{"ec2_iam_role"}, AssumeRoleEnabled: true})
		sess, err := cache.GetSession(defaultRegion, settings)
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
