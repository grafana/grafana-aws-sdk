package awsds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGrafanaAssumeRoleExternalID(t *testing.T) {
	trueVal := true
	falseVal := false

	assert.Equal(t, "stack-uid", ResolveGrafanaAssumeRoleExternalID(&trueVal, "stack-uid", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(&falseVal, "stack-uid", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(&trueVal, "", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(nil, "stack-uid", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(nil, "", "stack"))
}
