package awsds

import (
	"fmt"
	"os"
	"runtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// GetUserAgentString returns an agent that can be parsed in server logs
func GetUserAgentString(name string) string {
	// Build info is set from compile time flags
	buildInfo, err := build.GetBuildInfo()
	if err != nil {
		buildInfo.Version = "dev"
		buildInfo.Hash = "?"
	}

	if len(buildInfo.Hash) > 8 {
		buildInfo.Hash = buildInfo.Hash[0:8]
	}

	grafanaVersion := os.Getenv("GF_VERSION")
	if grafanaVersion == "" {
		grafanaVersion = "?"
	}

	return fmt.Sprintf("%s/%s (%s; %s;) %s/%s-%s Grafana/%s",
		aws.SDKName,
		aws.SDKVersion,
		runtime.Version(),
		runtime.GOOS,
		name,
		buildInfo.Version,
		buildInfo.Hash,
		grafanaVersion)
}

func getDatasourceUID(settings backend.DataSourceInstanceSettings) string {
	datasourceUID := settings.UID
	// Grafana < 8.0 won't include the UID yet
	if datasourceUID == "" {
		datasourceUID = fmt.Sprintf("%d", settings.ID)
	}
	return datasourceUID
}

// getErrorFrameFromQuery returns a error frames with empty data and meta fields
func getErrorFrameFromQuery(query *AsyncQuery) data.Frames {
	frames := data.Frames{}
	frame := data.NewFrame(query.RefID)
	frame.Meta = &data.FrameMeta{
		ExecutedQueryString: query.RawSQL,
	}
	frames = append(frames, frame)
	return frames
}
