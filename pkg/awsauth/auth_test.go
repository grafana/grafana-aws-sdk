package awsauth

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/grafana/grafana-aws-sdk-for-backport/pkg/awsds"
	"github.com/grafana/grafana-aws-sdk-for-backport/pkg/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"maps"
	"os"
	"testing"
	"time"
)

const StackID = "42"

var defaultGrafanaConfig = map[string]string{
	awsds.GrafanaAssumeRoleExternalIdKeyName: StackID,
	awsds.AllowedAuthProvidersEnvVarKeyName:  "keys,default,grafana_assume_role,credentials",
}

type testSuite []testCase

func (ts testSuite) runAll(t *testing.T) {
	for _, tc := range ts {
		t.Run(tc.name, tc.Run)
	}
}

type testCase struct {
	name                 string
	shouldError          bool
	authSettings         Settings
	assumedCredentials   *ststypes.Credentials
	assumeRoleShouldFail bool
	environment          map[string]string
	grafanaConfig        map[string]string
}

func (tc testCase) Run(t *testing.T) {
	grafanaCfg := maps.Clone(defaultGrafanaConfig)
	maps.Copy(grafanaCfg, tc.grafanaConfig)
	ctx := backend.WithGrafanaConfig(context.Background(), backend.NewGrafanaCfg(grafanaCfg))
	client := &mockAWSAPIClient{&mockAssumeRoleAPIClient{}}

	if tc.authSettings.AssumeRoleARN != "" {
		client.assumeRoleClient.On("AssumeRole").Return(tc.assumeRoleShouldFail, tc.assumedCredentials)
	}
	provider := newAWSConfigProviderWithClient(client)
	defer setUpAndRestoreEnvironment(tc.environment)() // a little goofy-looking but it works

	cfg, err := provider.GetConfig(ctx, tc.authSettings)

	if tc.shouldError {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(ctx)
		if tc.assumeRoleShouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			tc.assertConfig(t, cfg)
			if tc.authSettings.GetAuthType() == AuthTypeKeys && tc.authSettings.SessionToken != "" {
				assert.Equal(t, tc.authSettings.SessionToken, creds.SessionToken)
			}
			if tc.authSettings.GetAuthType() == AuthTypeGrafanaAssumeRole {
				assert.Equal(t, client.assumeRoleClient.calledExternalId, StackID)
			} else if tc.authSettings.AssumeRoleARN != "" && tc.authSettings.ExternalID != "" {
				assert.Equal(t, client.assumeRoleClient.calledExternalId, tc.authSettings.ExternalID)
			}
			accessKey, secret := tc.getExpectedKeyAndSecret(t)
			assert.Equal(t, accessKey, creds.AccessKeyID)
			assert.Equal(t, secret, creds.SecretAccessKey)
		}
	}
	if isStsEndpoint(&tc.authSettings.Endpoint) {
		assert.Nil(t, cfg.BaseEndpoint)
	}
}

func (tc testCase) assertConfig(t *testing.T, cfg aws.Config) {
	if tc.authSettings.GetAuthType() == AuthTypeDefault && tc.environment["AWS_REGION"] != "" {
		assert.Equal(t, tc.environment["AWS_REGION"], cfg.Region)
	} else {
		assert.Equal(t, tc.authSettings.Region, cfg.Region)
	}
}

func (tc testCase) getExpectedKeyAndSecret(t *testing.T) (string, string) {
	if tc.assumedCredentials != nil {
		return *tc.assumedCredentials.AccessKeyId, *tc.assumedCredentials.SecretAccessKey
	}
	switch tc.authSettings.GetAuthType() {
	case AuthTypeKeys:
		return tc.authSettings.AccessKey, tc.authSettings.SecretKey
	case AuthTypeSharedCreds:
		// from testdata/shared_credentials
		return "AFAKEONEYESGOOD", "zippitydoodah"
	case AuthTypeGrafanaAssumeRole:
		// from testdata/assume_role_credentials
		return "ADIFFERENTONENICE", "merrilywerollalong"
	case AuthTypeDefault:
		if tc.environment["AWS_ACCESS_KEY_ID"] != "" {
			return tc.environment["AWS_ACCESS_KEY_ID"], tc.environment["AWS_SECRET_ACCESS_KEY"]
		} else {
			// from testdata/credentials
			return "AREGULAROLDKEY", "askmenosecrets"
		}
	case AuthTypeEC2IAMRole:
		t.Error("This test type is not yet implemented")
		return "", ""
	default:
		t.Errorf("Unsupported auth type: %s", tc.authSettings.GetAuthType())
		return "", ""
	}
}

// setUpAndRestoreEnvironment sets the given environment variables and
// case and returns a function that restores them to their original value.
// Use like:
//
//	defer setUpAndRestoreEnvironment(env)()
func setUpAndRestoreEnvironment(env map[string]string) func() {
	origEnv := map[string]string{}
	for k, v := range env {
		origEnv[k] = os.Getenv(k)
		_ = os.Setenv(k, v)
	}
	return func() {
		for k, v := range origEnv {
			_ = os.Setenv(k, v)
		}
	}
}

func testDataPath(fn string) string {
	here, _ := os.Getwd()
	return fmt.Sprintf("%s/testdata/%s", here, fn)
}

func TestGetAWSConfig_Keys(t *testing.T) {
	testSuite{
		{
			name: "static credentials",
			authSettings: Settings{
				AuthType:  AuthTypeKeys,
				AccessKey: "tensile",
				SecretKey: "diaphanous",
				Region:    "eu-north-1",
			},
		},
		{
			name: "static credentials, legacy auth type",
			authSettings: Settings{
				LegacyAuthType: awsds.AuthTypeKeys,
				AccessKey:      "ubiquitous",
				SecretKey:      "malevolent",
				Region:         "ap-south-1",
			},
		},
		{
			name: "static credentials, sts endpoint",
			authSettings: Settings{
				LegacyAuthType: awsds.AuthTypeKeys,
				AccessKey:      "ubiquitous",
				SecretKey:      "malevolent",
				Region:         "ap-south-1",
			},
		},
		{
			name: "static credentials with session token",
			authSettings: Settings{
				LegacyAuthType: awsds.AuthTypeKeys,
				AccessKey:      "ubiquitous",
				SecretKey:      "malevolent",
				Region:         "ap-south-1",
				SessionToken:   "alphabet",
			},
		},
	}.runAll(t)
}

func TestGetAWSConfig_Keys_AssumeRule(t *testing.T) {
	testSuite{
		{
			name: "static assume role with success",
			authSettings: Settings{
				AuthType:      AuthTypeKeys,
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "eu-north-1",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
			},
			assumedCredentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("assumed"),
				SecretAccessKey: aws.String("role"),
				SessionToken:    aws.String("session"),
				Expiration:      aws.Time(time.Now().Add(time.Hour)),
			},
		},
		{
			name: "static assume role with external ID - external ID is used",
			authSettings: Settings{
				AuthType:      AuthTypeKeys,
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "eu-north-1",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
				ExternalID:    "cows_with_parasols",
			},
			assumedCredentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("assumed"),
				SecretAccessKey: aws.String("role"),
				SessionToken:    aws.String("session"),
				Expiration:      aws.Time(time.Now().Add(time.Hour)),
			},
		},
		{
			name: "static assume role with sts endpoint - endpoint is nil",
			authSettings: Settings{
				AuthType:      AuthTypeKeys,
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "us-east-1",
				Endpoint:      "sts.us-east-1.amazonaws.com",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
			},
			assumedCredentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("assumed"),
				SecretAccessKey: aws.String("role"),
				SessionToken:    aws.String("session"),
				Expiration:      aws.Time(time.Now().Add(time.Hour)),
			},
		},
		{
			name: "static assume role with sts endpoint - endpoint is nil",
			authSettings: Settings{
				AuthType:      AuthTypeKeys,
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "us-east-1",
				Endpoint:      "https://sts.us-east-1.amazonaws.com",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
			},
			assumedCredentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("assumed"),
				SecretAccessKey: aws.String("role"),
				SessionToken:    aws.String("session"),
				Expiration:      aws.Time(time.Now().Add(time.Hour)),
			},
		},
		{
			name: "static assume role with sts fips endpoint - endpoint is nil",
			authSettings: Settings{
				AuthType:      AuthTypeKeys,
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "us-east-1",
				Endpoint:      "sts-fips.us-east-1.amazonaws.com",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
			},
			assumedCredentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("assumed"),
				SecretAccessKey: aws.String("role"),
				SessionToken:    aws.String("session"),
				Expiration:      aws.Time(time.Now().Add(time.Hour)),
			},
		},
		{
			name: "static assume role with failure",
			authSettings: Settings{
				AuthType:      "keys",
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "eu-north-1",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
			},
			assumeRoleShouldFail: true,
		},
	}.runAll(t)
}

func TestGetAWSConfig_Default(t *testing.T) {
	testSuite{
		{
			name: "default reads from environment",
			authSettings: Settings{
				AuthType: AuthTypeDefault,
			},
			environment: map[string]string{
				"AWS_ACCESS_KEY_ID":     "something",
				"AWS_SECRET_ACCESS_KEY": "beautiful",
				"AWS_REGION":            "us-north-1",
			},
		},
		{
			name: "default reads from credentials file",
			authSettings: Settings{
				AuthType: "default",
			},
			environment: map[string]string{
				"AWS_SHARED_CREDENTIALS_FILE": testDataPath("credentials"),
			},
		},
	}.runAll(t)
}

func TestGetAWSConfig_AuthTypeNotAllowed(t *testing.T) {
	testSuite{
		{
			name: "don't allow auth types not in allowed list",
			authSettings: Settings{
				AuthType: AuthTypeKeys,
			},
			environment: map[string]string{
				"AWS_ACCESS_KEY_ID":     "something",
				"AWS_SECRET_ACCESS_KEY": "beautiful",
				"AWS_REGION":            "us-north-1",
			},
			grafanaConfig: map[string]string{
				awsds.AllowedAuthProvidersEnvVarKeyName: "default",
			},
			shouldError: true,
		},
		{
			name: "don't allow assume role if it is disabled",
			authSettings: Settings{
				AuthType:      AuthTypeDefault,
				AssumeRoleARN: "arn:whatever",
			},
			grafanaConfig: map[string]string{
				awsds.AllowedAuthProvidersEnvVarKeyName: "default",
				awsds.AssumeRoleEnabledEnvVarKeyName:    "false",
			},
			shouldError: true,
		},
	}.runAll(t)
}

func TestGetAWSConfig_Shared(t *testing.T) {
	testSuite{
		{
			name: "shared reads from specified file",
			authSettings: Settings{
				AuthType:           AuthTypeSharedCreds,
				CredentialsPath:    testDataPath("shared_credentials"),
				CredentialsProfile: "shared_profile",
			},
		},
		{
			name: "grafana assume role uses the shared mechanism",
			authSettings: Settings{
				AuthType:      AuthTypeGrafanaAssumeRole,
				AssumeRoleARN: "arn:aws:iam::1234567890:role/customer-role",
			},
			environment: map[string]string{
				"AWS_SHARED_CREDENTIALS_FILE": testDataPath("assume_role_credentials"),
			},
			assumedCredentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("horses"),
				SecretAccessKey: aws.String("unicorns"),
				SessionToken:    aws.String("riding"),
				Expiration:      aws.Time(time.Now().Add(time.Hour)),
			},
		},
	}.runAll(t)
}

func TestGetAWSConfig_UnknownOrMissing(t *testing.T) {
	testSuite{
		{
			name: "unknown auth type fails",
			authSettings: Settings{
				AuthType: AuthTypeUnknown,
			},
			shouldError: true,
		},
		{
			name: "random auth type fails",
			authSettings: Settings{
				AuthType: "rainbows",
			},
			shouldError: true,
		},
		{
			name:         "missing auth type fails back to legacy default (and does not fail)",
			authSettings: Settings{},
			environment: map[string]string{
				"AWS_SHARED_CREDENTIALS_FILE": testDataPath("credentials"),
			},
			shouldError: false,
		},
	}.runAll(t)
}
