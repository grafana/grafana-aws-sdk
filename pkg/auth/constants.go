package auth

// path to the shared credentials file in the instance for the aws/aws-sdk
// if empty string, the path is ~/.aws/credentials
const CredentialsPath = ""

// the profile containing credentials for GrafanaAssueRole auth type in the shared credentials file
const ProfileName = "assume_role_credentials"

// AllowedAuthProvidersEnvVarKeyName is the string literal for the aws allowed auth providers environment variable key name
const AllowedAuthProvidersEnvVarKeyName = "AWS_AUTH_AllowedAuthProviders"

// AssumeRoleEnabledEnvVarKeyName is the string literal for the aws assume role enabled environment variable key name
const AssumeRoleEnabledEnvVarKeyName = "AWS_AUTH_AssumeRoleEnabled"

// SessionDurationEnvVarKeyName is the string literal for the session duration variable key name
const SessionDurationEnvVarKeyName = "AWS_AUTH_SESSION_DURATION"

// GrafanaAssumeRoleExternalIdKeyName is the string literal for the grafana assume role external id environment variable key name
const GrafanaAssumeRoleExternalIdKeyName = "AWS_AUTH_EXTERNAL_ID"
