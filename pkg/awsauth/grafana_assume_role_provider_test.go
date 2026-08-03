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
	accessKeyPath, secretKeyPath := writeCredentialFiles(t, " access-key\n", "\tsecret-key\n")
	provider := newGrafanaAssumeRoleProviderWithExpiry(accessKeyPath, secretKeyPath, stscreds.DefaultDuration)
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
	accessKeyPath, secretKeyPath := writeCredentialFiles(t, "first-access-key", "first-secret-key")
	provider := newGrafanaAssumeRoleProviderWithExpiry(accessKeyPath, secretKeyPath, time.Millisecond)
	cache := aws.NewCredentialsCache(provider)

	first, err := cache.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "first-access-key", first.AccessKeyID)
	assert.Equal(t, "first-secret-key", first.SecretAccessKey)

	require.NoError(t, os.WriteFile(accessKeyPath, []byte("second-access-key"), 0o600))
	require.NoError(t, os.WriteFile(secretKeyPath, []byte("second-secret-key"), 0o600))
	time.Sleep(2 * time.Millisecond)

	second, err := cache.Retrieve(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "second-access-key", second.AccessKeyID)
	assert.Equal(t, "second-secret-key", second.SecretAccessKey)
}

func TestGrafanaAssumeRoleProviderReturnsReadError(t *testing.T) {
	dir := t.TempDir()
	provider := newGrafanaAssumeRoleProviderWithExpiry(
		filepath.Join(dir, "missing-access-key"),
		filepath.Join(dir, "missing-secret-key"),
		stscreds.DefaultDuration,
	)

	_, err := provider.Retrieve(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to read AWS access key file")
}

func writeCredentialFiles(t *testing.T, accessKey, secretKey string) (string, string) {
	t.Helper()

	dir := t.TempDir()
	accessKeyPath := filepath.Join(dir, "access-key-id")
	secretKeyPath := filepath.Join(dir, "secret-access-key")

	require.NoError(t, os.WriteFile(accessKeyPath, []byte(accessKey), 0o600))
	require.NoError(t, os.WriteFile(secretKeyPath, []byte(secretKey), 0o600))

	return accessKeyPath, secretKeyPath
}
