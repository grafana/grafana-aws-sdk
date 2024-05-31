# Grafana AWS SDK

This is a common package that can be used for all amazon plugins.

## Backend plugins (go sdk)

see the ./pkg folder

## Frontend configuration

Frontend code has been moved to https://github.com/grafana/grafana-aws-sdk-react

## Drone configuration

Drone signs the Drone configuration file. This needs to be run every time the drone.yml file is modified. See https://github.com/grafana/deployment_tools/blob/master/docs/infrastructure/drone/signing.md for more info.

### Update drone build

If you have not installed drone CLI follow [these instructions](https://docs.drone.io/cli/install/)

To sign the `.drone.yml` file:

```bash
# Get your drone token from https://drone.grafana.net/account
export DRONE_TOKEN=<Your DRONE_TOKEN>

mage drone
```
