# Local Dev

1. Navigate to whatever is consuming grafana-aws-sdk (ex: Grafana and/or an aws data source plugin)
2. In that repo find the go.mod file and add a replace line above the require line and point to the code path of your local copy of the repo: `replace github.com/grafana/grafana-aws-sdk => /Users/yourname/local/path/to/grafana-aws-sdk`

3. No additional build step is necessary, whatever consumes this repo will build it for you.

# Releasing:

1. Make a pr to update changelog with your changes and merge the changes to main
1. Navigate to https://github.com/grafana/grafana-aws-sdk/releases
1. Click the "Draft a new release" button
1. Click the "Choose a tag" dropdown and type in the name of the release you want (if the tag doesn't exist yet it will be created from whatever the target next to it is, in this case the default is main)
1. Type in a release title and description
1. Click Publish release
