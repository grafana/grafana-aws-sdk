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

type grafanaAssumeRoleProvider struct {
	accessKeyPath  string
	secretKeyPath  string
	expiryDuration time.Duration
	now            func() time.Time
}

func newGrafanaAssumeRoleProvider(accessKeyPath, secretKeyPath string) aws.CredentialsProvider {
	return newGrafanaAssumeRoleProviderWithExpiry(accessKeyPath, secretKeyPath, stscreds.DefaultDuration)
}

func newGrafanaAssumeRoleProviderWithExpiry(accessKeyPath, secretKeyPath string, expiryDuration time.Duration) *grafanaAssumeRoleProvider {
	if expiryDuration <= 0 {
		expiryDuration = stscreds.DefaultDuration
	}

	return &grafanaAssumeRoleProvider{
		accessKeyPath:  accessKeyPath,
		secretKeyPath:  secretKeyPath,
		expiryDuration: expiryDuration,
		now:            time.Now,
	}
}

func (p *grafanaAssumeRoleProvider) Retrieve(context.Context) (aws.Credentials, error) {
	accessKey, err := readCredentialFile(p.accessKeyPath)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to read AWS access key file: %w", err)
	}

	secretKey, err := readCredentialFile(p.secretKeyPath)
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
