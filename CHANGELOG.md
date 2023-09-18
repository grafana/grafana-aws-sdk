# Change Log

All notable changes to this project will be documented in this file.

## v0.19.2

- Update grafana-plugin-sdk-go from v0.134.0 to v0.172.0
- Update go from 1.17 to 1.20
- Add AMAZON_MANAGED_GRAFANA to the UserAgent string header

## v0.19.1

- Update aws-sdk from v1.44.9 to v1.44.323

## v0.19.0

- Add `il-central-1` to opt-in region list

## v0.18.0

- Add Support for Temporary Credentials in Grafana Cloud @idastambuk @sarahzinger [84](https://github.com/grafana/grafana-aws-sdk/pull/84)
- Add Contributing.md file

## v0.17.0

- Add GetDatasourceLastUpdatedTime util for client caching @iwysiu in [#90](https://github.com/grafana/grafana-aws-sdk/pull/90)

## v0.16.1

- ShouldCacheQuery should handle nil responses @iwysiu in [#87](https://github.com/grafana/grafana-aws-sdk/pull/87)

## v0.16.0

- Add ShouldCacheQuery util for async caching @iwysiu in [#85](https://github.com/grafana/grafana-aws-sdk/pull/85)

## v0.15.1

- Fix expressions with async datasource @iwysiu in [#83](https://github.com/grafana/grafana-aws-sdk/pull/83)

## v0.15.0

Updating opt-in regions list by @eunice98k in https://github.com/grafana/grafana-aws-sdk/pull/80

## v0.13.0

- Fix connections for multiple async datasources @iwysiu in [#73](https://github.com/grafana/grafana-aws-sdk/pull/73)
- Pass query args to GetAsyncDB @kevinwcyu in [#71](https://github.com/grafana/grafana-aws-sdk/pull/71)

## v0.12.0

Updating opt-in regions list by @robbierolin in https://github.com/grafana/grafana-aws-sdk/pull/66

## v0.11.0

Switch ec2 role cred provider to remote cred provider https://github.com/grafana/grafana-aws-sdk/pull/62

## v0.9.0

[Breaking Change] Refactor `GetSession` method to allow adding the data source config and the user agent to configure the default HTTP client.

## v0.8.0

Added interfaces and functions for SQL data sources
