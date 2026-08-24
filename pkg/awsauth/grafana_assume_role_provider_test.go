package awsauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrafanaAssumeRoleProviderRetrievesExpiringCredentials(t *testing.T) {
	paths := writeCredentialFiles(t, " access-key\n", "\tsecret-key\n")
	provider := newGrafanaAssumeRoleProviderWithExpiry(paths, stscreds.DefaultDuration)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time {
		return now
	}

	creds, err := provider.Retrieve(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "access-key", creds.AccessKeyID)
	assert.Equal(t, "secret-key", creds.SecretAccessKey)
	assert.Equal(t, grafanaAssumeRoleProviderSource, creds.Source)
	assert.True(t, creds.CanExpire)
	assert.Equal(t, now.Add(stscreds.DefaultDuration), creds.Expires)
}

func TestGrafanaAssumeRoleProviderRereadsFilesAfterCacheExpiry(t *testing.T) {
	paths := writeCredentialFiles(t, "first-access-key", "first-secret-key")
	provider := newGrafanaAssumeRoleProviderWithExpiry(paths, time.Millisecond)
	cache := aws.NewCredentialsCache(provider)

	first, err := cache.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "first-access-key", first.AccessKeyID)
	assert.Equal(t, "first-secret-key", first.SecretAccessKey)

	rotateCredentialFiles(t, paths, "second-access-key", "second-secret-key")
	time.Sleep(2 * time.Millisecond)

	second, err := cache.Retrieve(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "second-access-key", second.AccessKeyID)
	assert.Equal(t, "second-secret-key", second.SecretAccessKey)
}

func TestGrafanaAssumeRoleProviderCachesUntilExpiry(t *testing.T) {
	paths := writeCredentialFiles(t, "first-access-key", "first-secret-key")
	provider := newGrafanaAssumeRoleProviderWithExpiry(paths, time.Hour)
	cache := aws.NewCredentialsCache(provider)

	_, err := cache.Retrieve(context.Background())
	require.NoError(t, err)

	rotateCredentialFiles(t, paths, "second-access-key", "second-secret-key")

	second, err := cache.Retrieve(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "first-access-key", second.AccessKeyID)
}

func TestGrafanaAssumeRoleProviderDefaultsNonPositiveExpiry(t *testing.T) {
	paths := writeCredentialFiles(t, "access-key", "secret-key")

	provider := newGrafanaAssumeRoleProviderWithExpiry(paths, 0)

	assert.Equal(t, stscreds.DefaultDuration, provider.expiryDuration)
}

func TestGrafanaAssumeRoleProviderReturnsReadError(t *testing.T) {
	dir := t.TempDir()
	provider := newGrafanaAssumeRoleProviderWithExpiry(credentialFilePaths{
		accessKey: filepath.Join(dir, "missing-access-key"),
		secretKey: filepath.Join(dir, "missing-secret-key"),
	}, stscreds.DefaultDuration)

	_, err := provider.Retrieve(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to read AWS access key file")
}

func TestGrafanaAssumeRoleProviderRejectsEmptyCredentialFile(t *testing.T) {
	paths := writeCredentialFiles(t, "access-key", "   \n")
	provider := newGrafanaAssumeRoleProviderWithExpiry(paths, stscreds.DefaultDuration)

	_, err := provider.Retrieve(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to read AWS secret key file")
	assert.ErrorContains(t, err, "is empty")
}

func TestCredentialFilePathsExist(t *testing.T) {
	paths := writeCredentialFiles(t, "access-key", "secret-key")
	assert.True(t, paths.exist())

	require.NoError(t, os.Remove(paths.secretKey))
	assert.False(t, paths.exist())
}

// writeCredentialFiles lays out a temporary directory the same way the Kubernetes Secret volume
// is mounted at /tmp/aws.credentials, and returns the paths to it.
func writeCredentialFiles(t *testing.T, accessKey, secretKey string) credentialFilePaths {
	t.Helper()

	dir := t.TempDir()
	paths := credentialFilePaths{
		accessKey: filepath.Join(dir, "access-key-id"),
		secretKey: filepath.Join(dir, "secret-access-key"),
	}
	rotateCredentialFiles(t, paths, accessKey, secretKey)

	return paths
}

func rotateCredentialFiles(t *testing.T, paths credentialFilePaths, accessKey, secretKey string) {
	t.Helper()

	require.NoError(t, os.WriteFile(paths.accessKey, []byte(accessKey), 0o600))
	require.NoError(t, os.WriteFile(paths.secretKey, []byte(secretKey), 0o600))
}

// expireCredentialsCache forces the next Retrieve to go back to the underlying provider, standing
// in for the passage of the cache's expiry duration. TestGrafanaAssumeRoleProviderRereadsFilesAfterCacheExpiry
// covers the expiry timing itself.
func expireCredentialsCache(t *testing.T, provider aws.CredentialsProvider) {
	t.Helper()

	cache, ok := provider.(*aws.CredentialsCache)
	require.True(t, ok, "expected credentials to be wrapped in an *aws.CredentialsCache, got %T", provider)
	cache.Invalidate()
}

// useCredentialFiles points the package-level source credential paths at a temporary directory
// for the duration of the test.
func useCredentialFiles(t *testing.T, paths credentialFilePaths) {
	t.Helper()

	original := grafanaAssumeRoleSourceCredentials
	grafanaAssumeRoleSourceCredentials = paths
	t.Cleanup(func() {
		grafanaAssumeRoleSourceCredentials = original
	})
}
