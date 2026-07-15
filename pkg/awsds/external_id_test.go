package awsds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreferGrafanaExternalID(t *testing.T) {
	trueVal := true
	falseVal := false

	assert.True(t, PreferGrafanaExternalID(&trueVal, "stack-uid"))
	assert.False(t, PreferGrafanaExternalID(&falseVal, "stack-uid"))
	assert.False(t, PreferGrafanaExternalID(&falseVal, ""))
	assert.True(t, PreferGrafanaExternalID(nil, "stack-uid"))
	assert.False(t, PreferGrafanaExternalID(nil, ""))
}

func TestResolveGrafanaAssumeRoleExternalID(t *testing.T) {
	trueVal := true
	falseVal := false

	assert.Equal(t, "stack-uid", ResolveGrafanaAssumeRoleExternalID(&trueVal, "stack-uid", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(&falseVal, "stack-uid", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(&trueVal, "", "stack"))
	assert.Equal(t, "stack-uid", ResolveGrafanaAssumeRoleExternalID(nil, "stack-uid", "stack"))
	assert.Equal(t, "stack", ResolveGrafanaAssumeRoleExternalID(nil, "", "stack"))
}
