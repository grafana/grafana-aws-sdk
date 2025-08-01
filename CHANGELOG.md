# Change Log

All notable changes to this project will be documented in this file.

## 1.0.6

- Add support for auto-merging dependabot updates by @kevinwcyu in https://github.com/grafana/grafana-aws-sdk/pull/255
- Bump github.com/grafana/sqlds/v4 from 4.2.3 to 4.2.4 by @dependabot[bot] in https://github.com/grafana/grafana-aws-sdk/pull/271
- Add CloudWatch Network Monitor namespace by @jacobwoffenden in https://github.com/grafana/grafana-aws-sdk/pull/272
- Add Application Signals metrics by @iwysiu in https://github.com/grafana/grafana-aws-sdk/pull/273
- Mark unsupported auth type errors as downstream by @iwysiu in https://github.com/grafana/grafana-aws-sdk/pull/275

## 1.0.5

- Rerelease for internal reasons

## 1.0.4

- Fix: handle (and try to avoid) panic in sigv4 middleware by @njvrzm in [#269](https://github.com/grafana/grafana-aws-sdk/pull/269)
- Remove pr_commands by @kevinwcyu in [#266](https://github.com/grafana/grafana-aws-sdk/pull/266)

## 1.0.3

- Fix transport and cache bugs by @njvrzm in [#267](https://github.com/grafana/grafana-aws-sdk/pull/267)

## 1.0.2

- Add some missing auth config handling by @njvrzm in [#263](https://github.com/grafana/grafana-aws-sdk/pull/263)

## 1.0.1

- Fix: Use empty sha256 hash if request's GetBody method is nil by @njvrzm in [#262](https://github.com/grafana/grafana-aws-sdk/pull/262)

## 1.0.0

- Add SigV4 middleware for aws-sdk-go-v2 by @njvrzm in [#257](https://github.com/grafana/grafana-aws-sdk/pull/257)
- Remove aws-sdk-go v1 entirely )@njvrzm in [#258](https://github.com/grafana/grafana-aws-sdk/pull/258)
- Chore: Migrate to github actions for CI by @idastambuk in [#259](https://github.com/grafana/grafana-aws-sdk/pull/259)
- fix: bump to address vulnerabilities in go1.24.2 by @njvrzm in [#254](https://github.com/grafana/grafana-aws-sdk/pull/254)
- Add missing opt in regions (thailand/mexico/malaysia/taipei) by @chriscerie in [#250]
- Dependency updates:
  - Bump the all-go-dependencies group across 1 directory with 7 updates by @dependabot in [#256](https://github.com/grafana/grafana-aws-sdk/pull/256)

## 0.38.7

- Cloudwatch metrics: fix uppercase for metrics in [#252](https://github.com/grafana/grafana-aws-sdk/pull/252)

## 0.38.6

- Fix: use configured externalID when not using Grafana Assume Role by @njvrzm in [#248](https://github.com/grafana/grafana-aws-sdk/pull/248)

## 0.38.5

- Fix externalID handling by @njvrzm in [#246](https://github.com/grafana/grafana-aws-sdk/pull/246)
- Add CloudWatch AWS/EKS metrics and dimensions by @jangaraj in [#242](https://github.com/grafana/grafana-aws-sdk/pull/242)
- Bump github.com/grafana/grafana-plugin-sdk-go by @dependabot in [#241](https://github.com/grafana/grafana-aws-sdk/pull/241)

## 0.38.4

- Add AvailableMemory metric for the DMS service by @andriikushch in [#244](https://github.com/grafana/grafana-aws-sdk/pull/244)
- Github actions: Add token write permission by @idastambuk in [#243](https://github.com/grafana/grafana-aws-sdk/pull/243)
- Fix: Workspace IAM Role auth method does not need special handling by @njvrzm in [#245](https://github.com/grafana/grafana-aws-sdk/pull/245)

## 0.38.3

- Chore: add zizmore ignore rule in [#239](https://github.com/grafana/grafana-aws-sdk/pull/239)
- Bugfix: Sigv4: Use externalId when signing with sigv4 in [#238](https://github.com/grafana/grafana-aws-sdk/pull/238)
- Bump the all-go-dependencies with 9 updates, update go lint and vulnerability check image tags in [#232](https://github.com/grafana/grafana-aws-sdk/pull/232)
- Bump golang.org/x/net from 0.34.0 to 0.36.0 in the go_modules group in [#210](https://github.com/grafana/grafana-aws-sdk/pull/210)

## 0.38.2

- Support passing session token with v2 auth by @njvrzm in [#234](https://github.com/grafana/grafana-aws-sdk/pull/234)
- Add vault tokens and zizmor config by @katebrenner in [#236](https://github.com/grafana/grafana-aws-sdk/pull/236)
- Update network firewall metrics by @tristanburgess in [#237](https://github.com/grafana/grafana-aws-sdk/pull/237)

## 0.38.1

- Cleanup github actions files by @iwysiu in [#233](https://github.com/grafana/grafana-aws-sdk/pull/233)
- Fix check for multitenant temporary credentials by @iwysiu in [#235](https://github.com/grafana/grafana-aws-sdk/pull/235)

## 0.38.0

- Add support for multitenant temporary credentials to v1 path by @iwysiu in [#231](https://github.com/grafana/grafana-aws-sdk/pull/231)

## 0.37.0

- Fix: clone default transport instead of using it for PDC by @njvrzm in [#229](https://github.com/grafana/grafana-aws-sdk/pull/229)
- Fix paths for multitenant [#228](https://github.com/grafana/grafana-aws-sdk/pull/228)

## 0.36.0

- Add dimensions to msk connect and pipe metric namespaces by @rrhodes in [#223](https://github.com/grafana/grafana-aws-sdk/pull/223)
- Fix: Use DefaultClient in awsauth if given nil HTTPClient by @njvrzm in [#226](https://github.com/grafana/grafana-aws-sdk/pull/226)

## 0.35.0

- Update Namespace Metrics and Dimensions tests, add missing dimensions by @rrhodes in https://github.com/grafana/grafana-aws-sdk/pull/218
- Add DBLoadRelativeToNumVCPUs metric to RDS by @tristanburgess in https://github.com/grafana/grafana-aws-sdk/pull/219
- Add support for multi tenant temporary credentials by @iwysiu in https://github.com/grafana/grafana-aws-sdk/pull/213

## 0.34.0

- feat: Add metrics for lambda event source mappings by @rrhodes in https://github.com/grafana/grafana-aws-sdk/pull/216
- Enable dataproxy.row_limit configuration option from Grafana by @kevinwcyu in https://github.com/grafana/grafana-aws-sdk/pull/215

## 0.33.1

- Fix: use alternate STS endpoint for STS interaction if given by @njvrzm in https://github.com/grafana/grafana-aws-sdk/pull/214

## 0.33.0

- Update CodeBuild metrics and dimensions by @hectorruiz-it in https://github.com/grafana/grafana-aws-sdk/pull/209
- Add support for aws-sdk-go-v2 authentication by @njvrzm in https://github.com/grafana/grafana-aws-sdk/pull/202

## 0.32.0

- AWSDS: Add QueryExecutionError type

## 0.31.8

- Bump github.com/grafana/grafana-plugin-sdk-go from 0.265.0 to 0.266.0 in the all-go-dependencies group by @dependabot in https://github.com/grafana/grafana-aws-sdk/pull/204
- Bump the all-go-dependencies group across 1 directory with 2 updates by @dependabot in https://github.com/grafana/grafana-aws-sdk/pull/201
- Add missing LegacyModelInvocations AWS bedrock metric by @drmdrew in https://github.com/grafana/grafana-aws-sdk/pull/200
- Chore: add label to external contributions by @kevinwcyu in https://github.com/grafana/grafana-aws-sdk/pull/198
- Update CloudWatch AWS/EBS metrics and dimensions by @idastambuk in https://github.com/grafana/grafana-aws-sdk/pull/197
- Bump the all-go-dependencies group with 3 updates by @dependabot in https://github.com/grafana/grafana-aws-sdk/pull/194

## 0.31.7

- Bump the all-go-dependencies group across 1 directory with 4 updates by @dependabot in https://github.com/grafana/grafana-aws-sdk/pull/190
- Bump the all-go-dependencies group across 1 directory with 3 updates by @dependabot in https://github.com/grafana/grafana-aws-sdk/pull/193

## 0.31.6

- Add new SQS FIFO metrics by @thepalbi in https://github.com/grafana/grafana-aws-sdk/pull/187
- Add aws-sdk-go-v2 credentials provider (session wrapper) by @njvrzm in https://github.com/grafana/grafana-aws-sdk/pull/185

## 0.31.5

- Update dependencies in https://github.com/grafana/grafana-aws-sdk/pull/176
  - actions/checkout from 2 to 4
  - tibdex/github-app-token from 1.8.0 to 2.1.0
- Update github.com/grafana/sqlds/v4 from 4.1.2 to 4.1.3 in https://github.com/grafana/grafana-aws-sdk/pull/178
- Remove ReadAuthSettings deprecation warning in https://github.com/grafana/grafana-aws-sdk/pull/184
- Add metrics for elasticache serverless in https://github.com/grafana/grafana-aws-sdk/pull/183
- Update AWS/AmplifyHosting metrics in https://github.com/grafana/grafana-aws-sdk/pull/186

## 0.31.4

- Update dependencies in https://github.com/grafana/grafana-aws-sdk/pull/175
  - github.com/aws/aws-sdk-go from v1.51.31 to v1.55.5
  - github.com/grafana/grafana-plugin-sdk-go from v0.250.0 to v0.258.0
  - github.com/grafana/sqlds/v4 from v4.1.0 to v4.1.2
- Update AWS/SES metrics and dimensions in https://github.com/grafana/grafana-aws-sdk/pull/174

## 0.31.3

- Update CloudWatch Metrics for AWS IoT SiteWise in https://github.com/grafana/grafana-aws-sdk/pull/172

## 0.31.2

- Upgrade grafana-plugin-sdk-go to v0.250.0 in https://github.com/grafana/grafana-aws-sdk/pull/170

## 0.31.1

- Mark dowstream errors in sessions.go in https://github.com/grafana/grafana-aws-sdk/pull/169

## 0.31.0

- Update sqlds to v4.1.0 in https://github.com/grafana/grafana-aws-sdk/pull/166
- Add AmazonMWAA and missing Aurora RDS Metrics in https://github.com/grafana/grafana-aws-sdk/pull/165
- Add more metrics to the services in https://github.com/grafana/grafana-aws-sdk/pull/161

## 0.30.0

- Sort NamespaceMetricsMap by @andriikushch in https://github.com/grafana/grafana-aws-sdk/pull/156
- Add expected casing for AWS/Kafka TCPConnections by @kgeckhart in https://github.com/grafana/grafana-aws-sdk/pull/158
- Move AWS/DataLifeCycleManager metrics to AWS/EBS by @iwysiu in https://github.com/grafana/grafana-aws-sdk/pull/159

## 0.29.0

- Support errorsource by @njvrzm in https://github.com/grafana/grafana-aws-sdk/pull/155
- Add DatabaseCapacityUsageCountedForEvictPercentage for AWS/ElastiCache by @andriikushch in https://github.com/grafana/grafana-aws-sdk/pull/152
- Add some missing metrics to AWS/ElastiCache by @andriikushch in https://github.com/grafana/grafana-aws-sdk/pull/153

## 0.28.0

- Add SigV4MiddlewareWithAuthSettings and deprecate SigV4Middleware [#150](https://github.com/grafana/grafana-aws-sdk/pull/150)

[Breaking Change] `sigv4.New` now expects the auth settings to be passed in instead of fetched from environment variables.

## 0.27.1

- add case sensitive metric name millisBehindLatest for KinesisAnalytics by @tristanburgess in https://github.com/grafana/grafana-aws-sdk/pull/148

## v0.27.0

- Add GetSessionWithAuthSettings and Deprecate GetSession [#144](https://github.com/grafana/grafana-aws-sdk/pull/144)

## v0.26.1

- Add CloudWatch Metrics and Dimension Key maps by @iwysiu in [#142](https://github.com/grafana/grafana-aws-sdk/pull/142)

## v0.26.0

- **breaking**: Add more context handling @njvrzm in [#139](https://github.com/grafana/grafana-aws-sdk/pull/139)
- upgrade all deps by @tristanburgess in [#134](https://github.com/grafana/grafana-aws-sdk/pull/134)
- Cleanup: typos, unused methods & parameters, docstrings, etc. by @njvrzm in [#138](https://github.com/grafana/grafana-aws-sdk/pull/138)

## v0.25.1

- Fix: aws sts assume role with custom endpoint in [#136](https://github.com/grafana/grafana-aws-sdk/pull/136)

## v0.25.0

- Add SigV4 middleware from Grafana core.

## v0.24.0

- Sessions: Use STS regional endpoint in assume role for opt-in regions in [#129](https://github.com/grafana/grafana-aws-sdk/pull/129)
- Add health check for async queries in [#124](https://github.com/grafana/grafana-aws-sdk/pull/125)

## v0.23.1

-Fix warning for getting GF_SECURE_SOCKS_DATASOURCE_PROXY_SERVER_ENABLED env variable [#125](https://github.com/grafana/grafana-aws-sdk/pull/125)

## v0.23.0

- Deprecate using environment variables for auth settings in sessions [#121](https://github.com/grafana/grafana-aws-sdk/pull/121)

## v0.22.0

- Add ReadAuthSettings to get config settings from context [#118](https://github.com/grafana/grafana-aws-sdk/pull/118)

## v0.21.0

- Update grafana-plugin-sdk-go to v0.201.0
- Update sqlds to v3.2.0

## v0.20.0

- Add ca-west-1 to list of opt-in regions @zspeaks [#111](https://github.com/grafana/grafana-aws-sdk/pull/111)

## v0.19.3

- Fix assuming a role with an endpoint set
- Include invalid authType in error message
- Update go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace from 0.37.0 to 0.44.0

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
