import { SelectableValue } from '@grafana/data';
import { AwsAuthType } from './types';

export const awsAuthProviderOptions = [
  {
    label: 'Workspace IAM Role',
    value: AwsAuthType.EC2IAMRole,
  },
  {
    label: 'AWS SDK Default',
    value: AwsAuthType.Default,
  },
  {
    label: 'Access & secret key',
    value: AwsAuthType.Keys,
  },
  {
    label: 'Credentials file',
    value: AwsAuthType.Credentials,
  },
] as Array<SelectableValue<AwsAuthType>>;
