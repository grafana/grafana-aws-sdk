import { SelectableValue } from '@grafana/data';
import { AwsAuthType } from './types';

export const awsAuthProviderOptions = [
  {
    label: 'EC2 IAM Role',
    value: AwsAuthType.EC2IAMRole,
    description: 'The credentials for the IAM Role that is running the EC2 instance will be used',
  },
  {
    label: 'AWS SDK Default',
    value: AwsAuthType.Default,
    description:
      'The SDK uses the first provider in the default provider chain that returns credentials without an error',
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
