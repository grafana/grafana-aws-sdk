package awsds

import (
	"github.com/aws/aws-sdk-go/aws/session"
)

// AmazonSessionProvider will return a session (perhaps cached) for given region and settings
type AmazonSessionProvider func(region string, s DatasourceSettings) (*session.Session, error)
