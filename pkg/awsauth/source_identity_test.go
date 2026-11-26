package awsauth

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceIdentityFromUser(t *testing.T) {
	tests := []struct {
		name string
		user *backend.User
		want string
	}{
		{name: "no user", user: nil, want: ""},
		{name: "empty user", user: &backend.User{}, want: ""},
		{name: "login is used", user: &backend.User{Login: "jdoe", Email: "john@example.com"}, want: "jdoe"},
		{name: "email is the fallback", user: &backend.User{Email: "john.doe+x@example.com"}, want: "john.doe+x@example.com"},
		{name: "whitespace is trimmed", user: &backend.User{Login: "  jdoe  "}, want: "jdoe"},
		{name: "whitespace only", user: &backend.User{Login: "   "}, want: ""},
		{name: "allowed punctuation is kept", user: &backend.User{Login: "a_b+c=d,e.f@g-h"}, want: "a_b+c=d,e.f@g-h"},
		{name: "exactly 64 valid characters is kept", user: &backend.User{Login: strings.Repeat("b", 64)}, want: strings.Repeat("b", 64)},
		{name: "too short is dropped", user: &backend.User{Login: "a"}, want: ""},
		{name: "two characters is kept", user: &backend.User{Login: "ab"}, want: "ab"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.user != nil {
				ctx = backend.WithUser(ctx, tc.user)
			}
			assert.Equal(t, tc.want, SourceIdentityFromUser(ctx))
		})
	}
}

var stsSourceIdentityRE = regexp.MustCompile(`^[\w+=,.@-]{2,64}$`)

func TestSourceIdentityFromUser_NeverCollides(t *testing.T) {
	logins := []string{
		"j doe", "j_doe", "j/doe", "j:doe", "j doe ", // sanitised variants
		"ünï", "___", "üni", // non-ASCII
		strings.Repeat("a", 64), strings.Repeat("a", 65), strings.Repeat("a", 100), // truncation
		strings.Repeat("a", 64) + "x", strings.Repeat("a", 64) + "y",
		"alice@example.com", "alice@example.org",
	}
	seen := map[string]string{}
	for _, login := range logins {
		got := SourceIdentityFromUser(backend.WithUser(context.Background(), &backend.User{Login: login}))
		require.NotEmpty(t, got, login)
		assert.Regexp(t, stsSourceIdentityRE, got, "login %q", login)
		if prev, dup := seen[got]; dup && prev != strings.TrimSpace(login) {
			t.Errorf("logins %q and %q map to the same identity %q", prev, login, got)
		}
		seen[got] = strings.TrimSpace(login)
	}
}

func TestSourceIdentityFromUser_Deterministic(t *testing.T) {
	ctx := backend.WithUser(context.Background(), &backend.User{Login: "j doe/ünï:x"})
	first := SourceIdentityFromUser(ctx)
	assert.Equal(t, first, SourceIdentityFromUser(ctx))
	assert.True(t, strings.HasPrefix(first, "j_doe__n__x-"), first)
	assert.Equal(t, len("j_doe__n__x-")+8, len(first))
}

func TestSourceIdentityHashDiffers(t *testing.T) {
	base := Settings{AuthType: AuthTypeKeys, AccessKey: "a", SecretKey: "b", AssumeRoleARN: "arn:aws:iam::1:role/r"}
	alice, bob := base, base
	alice.SourceIdentity = "alice"
	bob.SourceIdentity = "bob"
	assert.NotEqual(t, base.Hash(), alice.Hash())
	assert.NotEqual(t, alice.Hash(), bob.Hash())
	assert.Equal(t, alice.Hash(), alice.Hash())
}

func newSourceIdentityTest(t *testing.T, settings Settings) (*awsConfigProvider, *mockAssumeRoleAPIClient, func(user *backend.User) context.Context) {
	t.Helper()
	assumeRole := &mockAssumeRoleAPIClient{}
	assumeRole.On("AssumeRole").Return(false, &ststypes.Credentials{
		AccessKeyId:     aws.String("assumed"),
		SecretAccessKey: aws.String("role"),
		SessionToken:    aws.String("session"),
		Expiration:      aws.Time(time.Now().Add(time.Hour)),
	})
	provider := newAWSConfigProviderWithClient(&mockAWSAPIClient{assumeRole})
	ctxFor := func(user *backend.User) context.Context {
		ctx := config.WithGrafanaConfig(context.Background(), config.NewGrafanaCfg(defaultGrafanaConfig))
		if user != nil {
			ctx = backend.WithUser(ctx, user)
		}
		return ctx
	}
	return provider, assumeRole, ctxFor
}

func assumeRoleSettings() Settings {
	return Settings{
		AuthType:      AuthTypeKeys,
		AccessKey:     "tensile",
		SecretKey:     "diaphanous",
		Region:        "eu-north-1",
		AssumeRoleARN: "arn:aws:iam::1234567890:role/aws-service-role",
	}
}

func retrieve(t *testing.T, ctx context.Context, p *awsConfigProvider, s Settings) {
	t.Helper()
	cfg, err := p.GetConfig(ctx, s)
	require.NoError(t, err)
	creds, err := cfg.Credentials.Retrieve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "assumed", creds.AccessKeyID)
}

func TestGetAWSConfig_SourceIdentityFromUser(t *testing.T) {
	t.Run("sets the SourceIdentity of the assume role call to the user", func(t *testing.T) {
		s := assumeRoleSettings()
		s.SourceIdentityFromUser = true
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		retrieve(t, ctxFor(&backend.User{Login: "alice@example.com"}), p, s)
		assert.Equal(t, []string{"alice@example.com"}, sts.sourceIdentities)
	})

	t.Run("does nothing when the option is off", func(t *testing.T) {
		s := assumeRoleSettings()
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		retrieve(t, ctxFor(&backend.User{Login: "alice"}), p, s)
		assert.Equal(t, []string{""}, sts.sourceIdentities)
	})

	t.Run("assumes the role without an identity when the request has no user", func(t *testing.T) {
		s := assumeRoleSettings()
		s.SourceIdentityFromUser = true
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		retrieve(t, ctxFor(nil), p, s)
		assert.Equal(t, []string{""}, sts.sourceIdentities)
	})

	t.Run("is ignored without an assume role ARN", func(t *testing.T) {
		s := assumeRoleSettings()
		s.AssumeRoleARN = ""
		s.SourceIdentityFromUser = true
		p, _, ctxFor := newSourceIdentityTest(t, s)
		cfg, err := p.GetConfig(ctxFor(&backend.User{Login: "alice"}), s)
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "tensile", creds.AccessKeyID)
	})

	t.Run("each user gets their own session and the same user reuses it", func(t *testing.T) {
		s := assumeRoleSettings()
		s.SourceIdentityFromUser = true
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		alice, bob := &backend.User{Login: "alice"}, &backend.User{Login: "bob"}
		retrieve(t, ctxFor(alice), p, s)
		retrieve(t, ctxFor(bob), p, s)
		retrieve(t, ctxFor(alice), p, s)
		retrieve(t, ctxFor(bob), p, s)
		assert.Equal(t, []string{"alice", "bob"}, sts.sourceIdentities, "one STS call per user, cached afterwards")
	})

	t.Run("a user-less request does not reuse another user's session", func(t *testing.T) {
		s := assumeRoleSettings()
		s.SourceIdentityFromUser = true
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		retrieve(t, ctxFor(&backend.User{Login: "alice"}), p, s)
		retrieve(t, ctxFor(nil), p, s)
		assert.Equal(t, []string{"alice", ""}, sts.sourceIdentities)
	})

	t.Run("a caller-supplied SourceIdentity cannot be used to impersonate", func(t *testing.T) {
		s := assumeRoleSettings()
		s.SourceIdentityFromUser = true
		s.SourceIdentity = "mallory"
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		retrieve(t, ctxFor(&backend.User{Login: "alice"}), p, s)
		assert.Equal(t, []string{"alice"}, sts.sourceIdentities)
	})

	t.Run("a stale SourceIdentity is cleared when the option is off", func(t *testing.T) {
		s := assumeRoleSettings()
		s.SourceIdentity = "mallory"
		p, sts, ctxFor := newSourceIdentityTest(t, s)
		retrieve(t, ctxFor(&backend.User{Login: "alice"}), p, s)
		assert.Equal(t, []string{""}, sts.sourceIdentities)
	})
}

func TestGetAWSConfig_SourceIdentityFromUser_EmailFallback(t *testing.T) {
	s := assumeRoleSettings()
	s.SourceIdentityFromUser = true
	p, sts, ctxFor := newSourceIdentityTest(t, s)
	retrieve(t, ctxFor(&backend.User{Email: "alice@example.com"}), p, s)
	assert.Equal(t, []string{"alice@example.com"}, sts.sourceIdentities)
}

func TestGetAWSConfig_SourceIdentityFromUser_GrafanaAssumeRole(t *testing.T) {
	useCredentialFiles(t, credentialFilePaths{
		accessKey: filepath.Join(t.TempDir(), "access-key-id"),
		secretKey: filepath.Join(t.TempDir(), "secret-access-key"),
	})
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", testDataPath("assume_role_credentials"))
	s := Settings{
		AuthType:               AuthTypeGrafanaAssumeRole,
		AssumeRoleARN:          "arn:aws:iam::1234567890:role/customer-role",
		SourceIdentityFromUser: true,
	}
	p, sts, ctxFor := newSourceIdentityTest(t, s)
	retrieve(t, ctxFor(&backend.User{Login: "alice"}), p, s)
	retrieve(t, ctxFor(&backend.User{Login: "bob"}), p, s)
	assert.Equal(t, []string{"alice", "bob"}, sts.sourceIdentities)
}

func TestSourceIdentityHintProvider(t *testing.T) {
	boom := errors.New("AccessDenied")
	_, err := sourceIdentityHintProvider{inner: failingProvider{boom}}.Retrieve(context.Background())
	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "sts:SetSourceIdentity")

	creds, err := sourceIdentityHintProvider{inner: credsProvider{aws.Credentials{AccessKeyID: "k"}}}.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "k", creds.AccessKeyID)
}

func TestGetAWSConfig_SourceIdentityFromUser_AssumeRoleFailureHasHint(t *testing.T) {
	s := assumeRoleSettings()
	s.SourceIdentityFromUser = true
	assumeRole := &mockAssumeRoleAPIClient{}
	assumeRole.On("AssumeRole").Return(true, (*ststypes.Credentials)(nil))
	p := newAWSConfigProviderWithClient(&mockAWSAPIClient{assumeRole})
	ctx := config.WithGrafanaConfig(context.Background(), config.NewGrafanaCfg(defaultGrafanaConfig))
	ctx = backend.WithUser(ctx, &backend.User{Login: "alice"})
	cfg, err := p.GetConfig(ctx, s)
	require.NoError(t, err)
	_, err = cfg.Credentials.Retrieve(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sts:SetSourceIdentity")

	// without a SourceIdentity the error is left alone
	s2 := assumeRoleSettings()
	cfg, err = newAWSConfigProviderWithClient(&mockAWSAPIClient{assumeRole}).GetConfig(ctx, s2)
	require.NoError(t, err)
	_, err = cfg.Credentials.Retrieve(ctx)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sts:SetSourceIdentity")
}

type failingProvider struct{ err error }

func (f failingProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{}, f.err
}

type credsProvider struct{ c aws.Credentials }

func (c credsProvider) Retrieve(context.Context) (aws.Credentials, error) { return c.c, nil }
