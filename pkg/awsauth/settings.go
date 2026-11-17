package awsauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/build/buildinfo"

	"github.com/aws/aws-sdk-go-v2/aws/middleware"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	smithymiddleware "github.com/aws/smithy-go/middleware"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-aws-sdk/pkg/common"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
)

const (
	// awsTempCredsAccessKey and awsTempCredsSecretKey are the files containing the
	awsTempCredsAccessKey = "/tmp/aws.credentials/access-key-id"
	awsTempCredsSecretKey = "/tmp/aws.credentials/secret-access-key"
	profileName           = "assume_role_credentials"
)

type ProxyType string

const (
	ProxyTypeNone ProxyType = "none"
	ProxyTypeEnv  ProxyType = "env" // default
	ProxyTypeUrl  ProxyType = "url"
)

func GetProxyTypeFromString(proxyType string) ProxyType {
	switch proxyType {
	case "none":
		return ProxyTypeNone
	case "env":
		return ProxyTypeEnv
	case "url":
		return ProxyTypeUrl
	default:
		return ProxyTypeEnv
	}
}

type PerDatasourceProxySettings struct {
	ProxyType     ProxyType
	ProxyUrl      string
	ProxyUsername string
	ProxyPassword string
}

// Settings carries configuration for authenticating with AWS
type Settings struct {
	AuthType AuthType
	// deprecated: use AuthType instead
	LegacyAuthType     awsds.AuthType
	AccessKey          string
	SecretKey          string
	Region             string
	CredentialsPath    string
	CredentialsProfile string
	AssumeRoleARN      string
	// Endpoint overrides the endpoint of the service clients built from the
	// returned config. It is not used for the STS calls that resolve credentials.
	Endpoint string
	// STSEndpoint overrides the STS endpoint used to assume AssumeRoleARN. It takes
	// priority over AWS_ENDPOINT_URL_STS. Ignored for AuthTypeGrafanaAssumeRole.
	STSEndpoint string
	ExternalID  string
	// GrafanaExternalID is the per-datasource external ID for
	// AuthTypeGrafanaAssumeRole. Used only when UsePerDatasourceExternalID is true.
	GrafanaExternalID string
	// UsePerDatasourceExternalID selects per-datasource vs stack external ID.
	// Nil/false → stack (legacy). True → use GrafanaExternalID when set.
	UsePerDatasourceExternalID *bool
	// SourceIdentityFromUser, when true, sets the STS SourceIdentity of the assumed
	// role session to the Grafana user making the request, so that AWS-side logs such
	// as CloudTrail can attribute API calls to a person instead of a shared role.
	// It only applies when AssumeRoleARN is set. The role's trust policy must allow
	// sts:SetSourceIdentity. When the request has no user (for example alerting or
	// other backend-initiated queries) no SourceIdentity is set and the role is assumed
	// as before, so to enforce attribution add a trust policy condition that requires
	// sts:SourceIdentity to be present. It only takes effect where the plugin copies it
	// into Settings (it is not read for SigV4 requests). Each distinct user adds one
	// cached config for the life of the provider.
	SourceIdentityFromUser bool
	// SourceIdentity is the resolved identity for this request. It is filled in by
	// the config provider from the request context when SourceIdentityFromUser is
	// true; callers should not set it directly.
	SourceIdentity string
	UserAgent      string
	SessionToken   string
	HTTPClient     *http.Client
	ProxyOptions   *proxy.Options

	PerDatasourceProxySettings *PerDatasourceProxySettings
}

// Hash returns a value suitable for caching the config associated with these settings
func (s Settings) Hash() uint64 {
	h := fnv.New64()
	// In theory all of these except for region will be moot, because if any of them
	// change the datasource instance will be recycled. However, to ensure no leakage
	// of credentials between instances, we check everything except proxy options.
	// If those change the datasource will definitely not be reused.
	// Terminate each field so that moving a value between adjacent fields
	// (e.g. Endpoint and STSEndpoint) changes the hash.
	write := func(v string) {
		_, _ = h.Write([]byte(v))
		_, _ = h.Write([]byte{0})
	}
	write(string(s.GetAuthType()))
	write(s.AccessKey)
	write(s.SecretKey)
	write(s.Region)
	write(s.CredentialsPath)
	write(s.CredentialsProfile)
	write(s.AssumeRoleARN)
	write(s.Endpoint)
	write(s.STSEndpoint)
	write(s.ExternalID)
	write(s.GrafanaExternalID)
	if s.UsePerDatasourceExternalID != nil && *s.UsePerDatasourceExternalID {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
	// Sessions with different identities must never share cached credentials.
	write(s.SourceIdentity)
	if s.PerDatasourceProxySettings != nil {
		write(string(s.PerDatasourceProxySettings.ProxyType))
		write(s.PerDatasourceProxySettings.ProxyUrl)
		write(s.PerDatasourceProxySettings.ProxyUsername)
		write(s.PerDatasourceProxySettings.ProxyPassword)
	}
	return h.Sum64()
}

func (s Settings) GetAuthType() AuthType {
	if s.AuthType != AuthTypeMissing {
		return s.AuthType
	}
	return fromLegacy(s.LegacyAuthType)
}

func (s Settings) BaseOptions() []LoadOptionsFunc {
	return []LoadOptionsFunc{s.WithRegion(), s.WithEndpoint(), s.WithHTTPClient(), s.WithUserAgent()}
}

func (s Settings) BaseOptionsWithAuthSettings(ctx context.Context, authSettings *awsds.AuthSettings) []LoadOptionsFunc {
	return []LoadOptionsFunc{s.WithRegion(), s.WithEndpoint(), s.WithHTTPClientFromAuthSettings(authSettings), s.WithUserAgent()}
}

// credentialOptions is BaseOptionsWithAuthSettings without the service endpoint.
// LoadOptions.BaseEndpoint applies to every client built from the config, including
// the STS clients used to resolve credentials (AssumeRole, web identity, SSO), so
// the service endpoint must stay out of any config those clients are built from.
func (s Settings) credentialOptions(authSettings *awsds.AuthSettings) []LoadOptionsFunc {
	return []LoadOptionsFunc{s.WithRegion(), s.withFIPS(), s.WithHTTPClientFromAuthSettings(authSettings), s.WithUserAgent()}
}

func (s Settings) WithRegion() LoadOptionsFunc {
	return func(opts *config.LoadOptions) error {
		if s.Region != "" && s.Region != "default" {
			opts.Region = s.Region
		}
		return nil
	}
}

func (s Settings) WithEndpoint() LoadOptionsFunc {
	withFIPS := s.withFIPS()
	return func(options *config.LoadOptions) error {
		if s.hasServiceEndpoint() {
			options.BaseEndpoint = s.Endpoint
		}
		return withFIPS(options)
	}
}

func (s Settings) withFIPS() LoadOptionsFunc {
	return func(options *config.LoadOptions) error {
		if isFIPSEndpoint(s.Endpoint) {
			options.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
		return nil
	}
}

func isFIPSEndpoint(ep string) bool {
	// TODO: add fips support as an toggle option
	return strings.Contains(ep, "-fips.")
}

func (s Settings) hasServiceEndpoint() bool {
	return s.Endpoint != "" && s.Endpoint != "default" && !isFIPSEndpoint(s.Endpoint) && !isStsEndpoint(&s.Endpoint)
}

func (s Settings) WithStaticCredentials(client AWSAPIClient) LoadOptionsFunc {
	return func(opts *config.LoadOptions) error {
		opts.Credentials = client.NewStaticCredentialsProvider(s.AccessKey, s.SecretKey, s.SessionToken)
		return nil
	}
}

// WithSharedCredentials returns a LoadOptionsFunc to initialize config from a credentials file
func (s Settings) WithSharedCredentials() LoadOptionsFunc {
	return func(options *config.LoadOptions) error {
		options.SharedConfigProfile = s.CredentialsProfile
		if s.CredentialsPath != "" {
			options.SharedCredentialsFiles = []string{s.CredentialsPath}
		}
		return nil
	}
}

// WithGrafanaAssumeRole returns a LoadOptionsFunc to initialize config for Grafana Assume Role
func (s Settings) WithGrafanaAssumeRole(ctx context.Context, client AWSAPIClient) LoadOptionsFunc {
	logger := backend.Logger.FromContext(ctx)
	if grafanaAssumeRoleSourceCredentials.exist() {
		logger.Debug("using mounted source credentials for Grafana Assume Role")
		return func(opts *config.LoadOptions) error {
			provider := newGrafanaAssumeRoleProvider(grafanaAssumeRoleSourceCredentials)
			opts.Credentials = client.NewCredentialsCache(provider)
			return nil
		}
	}

	// if we don't find the files assume it's running single tenant and use the credentials file
	logger.Debug("mounted source credentials not found for Grafana Assume Role, falling back to shared profile", "profile", profileName)
	return func(options *config.LoadOptions) error {
		options.SharedConfigProfile = profileName
		if s.CredentialsPath != "" {
			options.SharedCredentialsFiles = []string{s.CredentialsPath}
		}
		return nil
	}
}

func (s Settings) WithAssumeRole(cfg aws.Config, client AWSAPIClient, sessionDuration *time.Duration) LoadOptionsFunc {
	if stsEndpoint := s.assumeRoleSTSEndpoint(); stsEndpoint != "" {
		// The region is kept even for opt-in regions: a custom STS endpoint (e.g. a VPC
		// endpoint) is regional and rejects requests signed for another region.
		cfg.BaseEndpoint = aws.String(stsEndpoint)
		// The STS client prefers an endpoint from ConfigSources over cfg.BaseEndpoint when
		// AWS_ENDPOINT_URL_STS is set, so put it there too, first. That makes it win over the
		// env var, the same way Endpoint wins over AWS_ENDPOINT_URL_<SERVICE>. FIPS is disabled
		// because STS refuses a custom endpoint while FIPS is enabled, which a FIPS service
		// endpoint turns on.
		cfg.ConfigSources = append([]any{config.LoadOptions{
			BaseEndpoint:    stsEndpoint,
			UseFIPSEndpoint: aws.FIPSEndpointStateDisabled,
		}}, cfg.ConfigSources...)
	} else if common.IsOptInRegion(cfg.Region) {
		cfg.Region = "us-east-1"
	}
	stsClient := client.NewSTSClientFromConfig(cfg)
	provider := client.NewAssumeRoleProvider(stsClient, s.AssumeRoleARN, func(options *stscreds.AssumeRoleOptions) {
		if s.ExternalID != "" {
			options.ExternalID = aws.String(s.ExternalID)
		}
		if sessionDuration != nil {
			options.Duration = *sessionDuration
		}
		if s.SourceIdentity != "" {
			options.SourceIdentity = aws.String(s.SourceIdentity)
		}
	})
	if s.SourceIdentity != "" {
		provider = sourceIdentityHintProvider{inner: provider}
	}
	cache := client.NewCredentialsCache(provider)
	return func(options *config.LoadOptions) error {
		options.Credentials = cache
		return nil
	}
}

// assumeRoleSTSEndpoint returns the STS endpoint to assume AssumeRoleARN with, or "" to
// use the default one. STSEndpoint is ignored for Grafana Assume Role, so calls signed
// with Grafana Cloud's source credentials can't be sent to a host the user picks.
func (s Settings) assumeRoleSTSEndpoint() string {
	ep := strings.TrimSpace(s.STSEndpoint)
	if ep == "" || s.GetAuthType() == AuthTypeGrafanaAssumeRole {
		return ""
	}
	return withDefaultScheme(ep)
}

// withDefaultScheme prefixes ep with https:// when it has no scheme, since the SDK
// rejects a bare host as an endpoint.
func withDefaultScheme(ep string) string {
	if strings.Contains(ep, "://") {
		return ep
	}
	return "https://" + ep
}

func (s Settings) WithEC2RoleCredentials(client AWSAPIClient) LoadOptionsFunc {
	return func(options *config.LoadOptions) error {
		options.Credentials = client.NewEC2RoleCreds()
		return nil
	}
}

func (s Settings) WithHTTPClient() LoadOptionsFunc {
	return s.WithHTTPClientFromAuthSettings(nil)
}

func (s Settings) WithHTTPClientFromAuthSettings(authSettings *awsds.AuthSettings) LoadOptionsFunc {
	return func(options *config.LoadOptions) error {
		if s.HTTPClient != nil {
			options.HTTPClient = s.HTTPClient
		}
		if options.HTTPClient == nil {
			client, err := httpclient.New()
			if err != nil {
				return err
			}
			options.HTTPClient = client
		}

		// only set the datasource level http proxy if the feature flag is enabled and the proxy type is not env
		setDatasourceLevelHTTPProxy := authSettings != nil && authSettings.PerDatasourceHTTPProxyEnabled && s.PerDatasourceProxySettings != nil && (s.PerDatasourceProxySettings.ProxyType != ProxyTypeEnv)
		if s.ProxyOptions != nil || setDatasourceLevelHTTPProxy {
			if client, ok := options.HTTPClient.(*http.Client); ok {
				if client.Transport == nil {
					client.Transport = httpclient.NewHTTPTransport()
				}
				if transport, ok := client.Transport.(*http.Transport); ok {
					// handle datasource level proxy url
					if setDatasourceLevelHTTPProxy {
						switch s.PerDatasourceProxySettings.ProxyType {
						case ProxyTypeUrl:
							u, err := GetProxyUrl(*s.PerDatasourceProxySettings)
							if err != nil {
								return err
							}
							transport.Proxy = http.ProxyURL(u)
						case ProxyTypeNone:
							transport.Proxy = http.ProxyURL(nil)
						default:
							// This is the default behavior, so we don't need to do anything
						}
					}

					// handle secure socks proxy
					if s.ProxyOptions != nil {
						err := proxy.New(s.ProxyOptions).ConfigureSecureSocksHTTPProxy(transport)
						if err != nil {
							return fmt.Errorf("error configuring Secure Socks proxy for Transport: %w", err)
						}
					}
				} else {
					return fmt.Errorf("cfg.HTTPClient.Transport is %T not *http.Transport", client.Transport)
				}
			} else {
				return fmt.Errorf("cfg.HTTPClient is not *http.Client")
			}
		}
		return nil
	}
}

func GetProxyUrl(settings PerDatasourceProxySettings) (*url.URL, error) {
	u, err := url.Parse(settings.ProxyUrl)
	if err != nil {
		return nil, backend.DownstreamError(err)
	}
	if settings.ProxyUsername != "" && settings.ProxyPassword != "" {
		u.User = url.UserPassword(settings.ProxyUsername, settings.ProxyPassword)
	}
	return u, nil
}

// WithUserAgent adds info to the UserAgent header of API requests.
// Adapted from grafana-aws-sdk/pkg/awsds/utils.go
func (s Settings) WithUserAgent() LoadOptionsFunc {
	version := "dev"
	buildInfo, err := buildinfo.GetBuildInfo()
	if err == nil {
		version = buildInfo.Version
	}
	grafanaVersion := os.Getenv("GF_VERSION")
	if grafanaVersion == "" {
		grafanaVersion = "?"
	}
	_, amgEnv := os.LookupEnv("AMAZON_MANAGED_GRAFANA")

	return func(options *config.LoadOptions) error {
		apiOpts := []func(*smithymiddleware.Stack) error{
			middleware.AddUserAgentKeyValue(aws.SDKName, aws.SDKVersion),
			middleware.AddUserAgentKey(fmt.Sprintf("(%s; %s;)", runtime.Version(), runtime.GOOS)),
		}
		if s.UserAgent != "" {
			apiOpts = append(apiOpts, middleware.AddUserAgentKeyValue(s.UserAgent, version))
		}
		apiOpts = append(apiOpts,
			middleware.AddUserAgentKeyValue("Grafana", grafanaVersion),
			middleware.AddUserAgentKeyValue("AMG", strconv.FormatBool(amgEnv)),
		)
		options.APIOptions = append(options.APIOptions, apiOpts...)
		return nil
	}
}

const (
	minSourceIdentityLength = 2
	maxSourceIdentityLength = 64
)

// SourceIdentityFromUser derives an STS SourceIdentity from the Grafana user in ctx.
// It prefers the login and falls back to the email. STS only accepts letters, digits
// and _+=,.@- (2 to 64 characters). An identity that is already valid is used as is.
// Otherwise unsupported characters are replaced with "_" and, because that mapping (and
// truncation to 64 characters) is lossy, a short hash of the original is appended so two
// different users never end up with the same identity. It returns "" when there is no
// usable user.
func SourceIdentityFromUser(ctx context.Context) string {
	user := backend.UserFromContext(ctx)
	if user == nil {
		return ""
	}
	raw := strings.TrimSpace(user.Login)
	if raw == "" {
		raw = strings.TrimSpace(user.Email)
	}
	if len(raw) < minSourceIdentityLength {
		return ""
	}
	var b strings.Builder
	changed := false
	for _, r := range raw {
		if isSourceIdentityRune(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
			changed = true
		}
	}
	out := b.String()
	if !changed && len(out) <= maxSourceIdentityLength {
		return out
	}
	sum := sha256.Sum256([]byte(raw))
	suffix := "-" + hex.EncodeToString(sum[:4])
	if len(out) > maxSourceIdentityLength-len(suffix) {
		out = out[:maxSourceIdentityLength-len(suffix)]
	}
	return out + suffix
}

func isSourceIdentityRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("_+=,.@-", r)
}

// sourceIdentityHintProvider adds a hint to credential errors when a SourceIdentity
// was requested, because the most likely cause is a trust policy that does not allow it.
type sourceIdentityHintProvider struct {
	inner aws.CredentialsProvider
}

func (p sourceIdentityHintProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := p.inner.Retrieve(ctx)
	if err != nil {
		return creds, fmt.Errorf("%w (a SourceIdentity was set for this session; the role's trust policy must allow sts:SetSourceIdentity)", err)
	}
	return creds, nil
}
