package awsauth

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

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
}

func (tc testCase) Run(t *testing.T) {
	ctx := context.Background()
	client := &mockAWSAPIClient{assumeRoleClient: &mockAssumeRoleAPIClient{}}
	defer setUpAndRestoreEnvironment(tc.environment)() // a little goofy-looking but it works

	if tc.authSettings.AssumeRoleARN != "" {
		client.assumeRoleClient.On("AssumeRole").Return(tc.assumeRoleShouldFail, tc.assumedCredentials)
	}

	cfg, err := getAWSConfigWithClient(ctx, tc.authSettings, client)

	if tc.shouldError {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
		tc.assertConfig(t, cfg)
		creds, _ := cfg.Credentials.Retrieve(ctx)
		accessKey, secret := tc.getExpectedKeyAndSecret(t)
		assert.Equal(t, accessKey, creds.AccessKeyID)
		assert.Equal(t, secret, creds.SecretAccessKey)
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
			name: "static assume role with failure",
			authSettings: Settings{
				AuthType:      "keys",
				AccessKey:     "tensile",
				SecretKey:     "diaphanous",
				Region:        "eu-north-1",
				AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
			},
			assumeRoleShouldFail: true,
			shouldError:          true,
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
			name: "shared reads from specified file",
			authSettings: Settings{
				AuthType: AuthTypeUnknown,
			},
			shouldError: true,
		},
		{
			name: "grafana assume role uses the shared mechanism",
			authSettings: Settings{
				AuthType: AuthTypeMissing,
			},
			shouldError: true,
		},
		{
			name: "grafana assume role uses the shared mechanism",
			authSettings: Settings{
				AuthType: "rainbows",
			},
			shouldError: true,
		},
	}.runAll(t)
}

func TestGetAWSConfig_EC2IAMRole(t *testing.T) {
	// TODO
	t.Skip()
}

type mockAssumeRoleAPIClient struct {
	mock.Mock
}

func (m *mockAssumeRoleAPIClient) AssumeRole(_ context.Context, params *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	args := m.Called()
	if args.Bool(0) { // shouldError
		return &sts.AssumeRoleOutput{}, fmt.Errorf("you can't do that")
	}
	return &sts.AssumeRoleOutput{
		AssumedRoleUser: &ststypes.AssumedRoleUser{
			Arn:           params.RoleArn,
			AssumedRoleId: aws.String("auto-generated-id"),
		},
		Credentials: args.Get(1).(*ststypes.Credentials),
	}, nil
}

type mockAWSAPIClient struct {
	mock.Mock

	assumeRoleClient *mockAssumeRoleAPIClient
}

func (m *mockAWSAPIClient) LoadDefaultConfig(ctx context.Context, options ...LoadOptionsFunc) (aws.Config, error) {
	opts := []LoadOptionsFunc{func(opts *config.LoadOptions) error {
		// Disable using EC2 instance metadata in config loading
		opts.EC2IMDSClientEnableState = imds.ClientDisabled
		// Disable endpoint discovery to avoid API calls out from tests
		opts.EnableEndpointDiscovery = aws.EndpointDiscoveryDisabled
		return nil
	}}
	opts = append(opts, options...)
	return config.LoadDefaultConfig(ctx, opts...)
}

func (m *mockAWSAPIClient) NewStaticCredentialsProvider(key, secret, session string) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(key, secret, session)
}

func (m *mockAWSAPIClient) NewSTSClientFromConfig(_ aws.Config) stscreds.AssumeRoleAPIClient {
	return m.assumeRoleClient
}

func (m *mockAWSAPIClient) NewAssumeRoleProvider(client stscreds.AssumeRoleAPIClient, arn string, opts ...func(*stscreds.AssumeRoleOptions)) aws.CredentialsProvider {
	return stscreds.NewAssumeRoleProvider(client, arn, opts...)
}

func (m *mockAWSAPIClient) NewCredentialsCache(provider aws.CredentialsProvider, optFns ...func(options *aws.CredentialsCacheOptions)) aws.CredentialsProvider {
	return aws.NewCredentialsCache(provider, optFns...)
}

func (m *mockAWSAPIClient) NewEC2RoleCreds() aws.CredentialsProvider {
	// TODO
	panic("not implemented")
}
