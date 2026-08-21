package awsauth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
)

const grafanaAssumeRoleProviderSource = "GrafanaAssumeRoleProvider"

// credentialFilePaths points at the mounted IAM user credentials that Grafana Cloud uses as
// the source identity for Grafana Assume Role.
type credentialFilePaths struct {
	accessKey string
	secretKey string
}

func (p credentialFilePaths) exist() bool {
	_, keyErr := os.Stat(p.accessKey)
	_, secretErr := os.Stat(p.secretKey)
	return keyErr == nil && secretErr == nil
}

// grafanaAssumeRoleSourceCredentials is a var rather than a const so tests can point it at a
// temporary directory.
var grafanaAssumeRoleSourceCredentials = credentialFilePaths{
	accessKey: awsTempCredsAccessKey,
	secretKey: awsTempCredsSecretKey,
}

// grafanaAssumeRoleProvider reads the source credentials from disk on every refresh rather than
// once at startup. The two IAM keys behind these files rotate about two weeks apart and their
// validity windows only partly overlap, so returning expiring credentials forces the AWS SDK
// credentials cache to re-read the files. Without that, a long-lived process keeps using
// whatever was on disk when it started and begins failing AssumeRole once that key is retired.
type grafanaAssumeRoleProvider struct {
	paths          credentialFilePaths
	expiryDuration time.Duration
	now            func() time.Time
}

func newGrafanaAssumeRoleProvider(paths credentialFilePaths) *grafanaAssumeRoleProvider {
	return newGrafanaAssumeRoleProviderWithExpiry(paths, stscreds.DefaultDuration)
}

func newGrafanaAssumeRoleProviderWithExpiry(paths credentialFilePaths, expiryDuration time.Duration) *grafanaAssumeRoleProvider {
	if expiryDuration <= 0 {
		expiryDuration = stscreds.DefaultDuration
	}

	return &grafanaAssumeRoleProvider{
		paths:          paths,
		expiryDuration: expiryDuration,
		now:            time.Now,
	}
}

func (p *grafanaAssumeRoleProvider) Retrieve(context.Context) (aws.Credentials, error) {
	accessKey, err := readCredentialFile(p.paths.accessKey)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to read AWS access key file: %w", err)
	}

	secretKey, err := readCredentialFile(p.paths.secretKey)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to read AWS secret key file: %w", err)
	}

	return aws.Credentials{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Source:          grafanaAssumeRoleProviderSource,
		CanExpire:       true,
		Expires:         p.now().Add(p.expiryDuration),
	}, nil
}

func readCredentialFile(path string) (string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return "", fmt.Errorf("credential file %q is empty", path)
	}

	return trimmed, nil
}
