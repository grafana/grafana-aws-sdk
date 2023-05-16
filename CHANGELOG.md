# Change Log

All notable changes to this project will be documented in this file.

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
