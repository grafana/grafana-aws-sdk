import { DataSourceJsonData, SelectableValue } from '@grafana/data';

export enum AwsAuthType {
  Keys = 'keys',
  Credentials = 'credentials',
  Default = 'default', // was 'arn',
  EC2IAMRole = 'ec2_IAM_role',
  /**
   * @deprecated use default
   */
  ARN = 'arn',
}

export interface AwsDataSourceJsonData extends DataSourceJsonData {
  authType?: AwsAuthType;
  assumeRoleArn?: string;
  externalId?: string;
  profile?: string; // Credentials profile name, as specified in ~/.aws/credentials
  defaultRegion?: string; // region if it is not defined by your credentials file
  endpoint?: string;
}

export interface AwsDataSourceSecureJsonData {
  accessKey?: string;
  secretKey?: string;
}

export const awsAuthProviderOptions = [
  {
    label: 'EC2 IAM Role',
    value: AwsAuthType.EC2IAMRole,
    description: 'The credentials for the IAM Role that is running the EC2 instance will be used',
  },
  {
    label: 'AWS SDK Default',
    value: AwsAuthType.Default,
    description: `
    The SDK uses the first provider in the chain that returns credentials without an error. 
    The default provider chain looks for credentials in the following order:
    1. Environment variables.
    2. Shared credentials file.
    3. If your application uses an ECS task definition or RunTask API operation, IAM role for tasks.
    4. If your application is running on an Amazon EC2 instance, IAM role for Amazon EC2.
  `,
  },
  {
    label: 'Access & secret key',
    value: AwsAuthType.Keys,
    description: `Uses the given access key ID and secret key to authenticate using the static credential provider in the AWS Golang SDK. 
    This provider doesn’t have any fallbacks, and will fail if the provided key pair doesn’t work. `,
  },
  {
    label: 'Credentials file',
    value: AwsAuthType.Credentials,
    description: `The application will attempt to read an AWS credentials file from the server.  
    This option allows you to specify which profile to use without using environment variables.`,
  },
] as Array<SelectableValue<AwsAuthType>>;
