import React, { FC, ReactNode, useEffect, useState } from 'react';
import { Input, Select, InlineField, ButtonGroup, ToolbarButton, FieldSet } from '@grafana/ui';
import {
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceJsonDataOptionSelect,
  onUpdateDatasourceResetOption,
  onUpdateDatasourceJsonDataOption,
  onUpdateDatasourceSecureJsonDataOption,
} from '@grafana/data';

import { standardRegions } from './regions';
import { AwsAuthDataSourceJsonData, AwsAuthDataSourceSecureJsonData, AwsAuthType } from './types';
import { awsAuthProviderOptions } from './providers';

const toOption = (value: string) => ({ value, label: value });

// TODO: move to grafana/ui
const LibraryCredential = ({ credentialName, children }: { credentialName: string; children: ReactNode }) => {
  return (
    <>
      <span data-lib-credential={credentialName}>{children}</span>
    </>
  );
};

export interface ConnectionConfigProps<J = AwsAuthDataSourceJsonData, S = AwsAuthDataSourceSecureJsonData>
  extends DataSourcePluginOptionsEditorProps<J, S> {
  standardRegions?: string[];
  loadRegions?: () => Promise<string[]>;
  defaultEndpoint?: string;
  skipHeader?: boolean;
  skipEndpoint?: boolean;
  children?: React.ReactNode;
}

export const ConnectionConfig: FC<ConnectionConfigProps> = (props: ConnectionConfigProps) => {
  const [regions, setRegions] = useState((props.standardRegions || standardRegions).map(toOption));
  const { loadRegions, onOptionsChange, skipHeader = false, skipEndpoint = false } = props;
  const options = props.options;
  let profile = options.jsonData.profile;
  if (profile === undefined) {
    profile = options.database;
  }

  const settings = (window as any).grafanaBootData.settings;
  const awsAllowedAuthProviders = settings.awsAllowedAuthProviders ?? [
    AwsAuthType.Default,
    AwsAuthType.Keys,
    AwsAuthType.Credentials,
  ];
  const awsAssumeRoleEnabled = settings.awsAssumeRoleEnabled ?? true;

  const currentProvider = awsAuthProviderOptions.find((p) => p.value === options.jsonData.authType);

  useEffect(() => {
    // Make sure a authType exists in the current model
    if (!currentProvider && awsAllowedAuthProviders.length) {
      onOptionsChange({
        ...options,
        jsonData: {
          ...options.jsonData,
          authType: awsAllowedAuthProviders[0],
        },
      });
    }
  }, [currentProvider, options, onOptionsChange]);

  useEffect(() => {
    if (!loadRegions) {
      return;
    }

    loadRegions().then((regions) => setRegions(regions.map(toOption)));
  }, [loadRegions]);

  return (
    <FieldSet label={skipHeader ? '' : 'Connection Details'} data-testid="connection-config">
      <InlineField
        label="Authentication Provider"
        labelWidth={28}
        tooltip="Specify which AWS credentials chain to use."
      >
        <LibraryCredential credentialName="authType">
          <Select
            className="width-30"
            value={currentProvider}
            options={awsAuthProviderOptions.filter((opt) => awsAllowedAuthProviders.includes(opt.value!))}
            defaultValue={options.jsonData.authType}
            onChange={(option) => {
              onUpdateDatasourceJsonDataOptionSelect(props, 'authType')(option);
            }}
          />
        </LibraryCredential>
      </InlineField>
      {options.jsonData.authType === 'credentials' && (
        <InlineField
          label="Credentials Profile Name"
          labelWidth={28}
          tooltip="Credentials profile name, as specified in ~/.aws/credentials, leave blank for default."
        >
          <LibraryCredential credentialName="profile">
            <Input
              className="width-30"
              placeholder="default"
              value={profile}
              onChange={onUpdateDatasourceJsonDataOption(props, 'profile')}
            />
          </LibraryCredential>
        </InlineField>
      )}

      {options.jsonData.authType === 'keys' && (
        <>
          <InlineField label="Access Key ID" labelWidth={28}>
            <LibraryCredential credentialName="accessKey">
              {props.options.secureJsonFields?.accessKey ? (
                <ButtonGroup className="width-30">
                  <Input disabled placeholder="Configured" />
                  <ToolbarButton
                    icon="edit"
                    tooltip="Edit Access Key ID"
                    type="button"
                    onClick={onUpdateDatasourceResetOption(props as any, 'accessKey')}
                  />
                </ButtonGroup>
              ) : (
                <Input
                  className="width-30"
                  value={options.secureJsonData?.accessKey ?? ''}
                  onChange={onUpdateDatasourceSecureJsonDataOption(props, 'accessKey')}
                />
              )}
            </LibraryCredential>
          </InlineField>

          <InlineField label="Secret Access Key" labelWidth={28}>
            <LibraryCredential credentialName="secretKey">
              {props.options.secureJsonFields?.secretKey ? (
                <ButtonGroup className="width-30">
                  <Input disabled placeholder="Configured" />
                  <ToolbarButton
                    icon="edit"
                    type="button"
                    tooltip="Edit Secret Access Key"
                    onClick={onUpdateDatasourceResetOption(props as any, 'secretKey')}
                  />
                </ButtonGroup>
              ) : (
                <Input
                  className="width-30"
                  value={options.secureJsonData?.secretKey ?? ''}
                  onChange={onUpdateDatasourceSecureJsonDataOption(props, 'secretKey')}
                />
              )}
            </LibraryCredential>
          </InlineField>
        </>
      )}

      {awsAssumeRoleEnabled && (
        <>
          <InlineField
            label="Assume Role ARN"
            labelWidth={28}
            tooltip="Optionally, specify the ARN of a role to assume. Specifying a role here will ensure that the selected authentication provider is used to assume the specified role rather than using the credentials directly. Leave blank if you don't need to assume a role at all"
          >
            <LibraryCredential credentialName="assumeRoleArn">
              <Input
                className="width-30"
                placeholder="arn:aws:iam:*"
                value={options.jsonData.assumeRoleArn || ''}
                onChange={onUpdateDatasourceJsonDataOption(props, 'assumeRoleArn')}
              />
            </LibraryCredential>
          </InlineField>
          <InlineField
            label="External ID"
            labelWidth={28}
            tooltip="If you are assuming a role in another account, that has been created with an external ID, specify the external ID here."
          >
            <LibraryCredential credentialName="externalId">
              <Input
                className="width-30"
                placeholder="External ID"
                value={options.jsonData.externalId || ''}
                onChange={onUpdateDatasourceJsonDataOption(props, 'externalId')}
              />
            </LibraryCredential>
          </InlineField>
        </>
      )}
      {!skipEndpoint && (
        <InlineField label="Endpoint" labelWidth={28} tooltip="Optionally, specify a custom endpoint for the service">
          <LibraryCredential credentialName="endpoint">
            <Input
              className="width-30"
              placeholder={props.defaultEndpoint ?? 'https://{service}.{region}.amazonaws.com'}
              value={options.jsonData.endpoint || ''}
              onChange={onUpdateDatasourceJsonDataOption(props, 'endpoint')}
            />
          </LibraryCredential>
        </InlineField>
      )}
      <InlineField
        label="Default Region"
        labelWidth={28}
        tooltip="Specify the region, such as for US West (Oregon) use ` us-west-2 ` as the region."
      >
        <LibraryCredential credentialName="defaultRegion">
          <Select
            className="width-30"
            value={regions.find((region) => region.value === options.jsonData.defaultRegion)}
            options={regions}
            defaultValue={options.jsonData.defaultRegion}
            allowCustomValue={true}
            onChange={onUpdateDatasourceJsonDataOptionSelect(props, 'defaultRegion')}
            formatCreateLabel={(r) => `Use region: ${r}`}
          />
        </LibraryCredential>
      </InlineField>
      {props.children}
    </FieldSet>
  );
};
