package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type AuthType int

const (
	AuthTypeDefault AuthType = iota
	AuthTypeSharedCreds
	AuthTypeKeys
	AuthTypeEC2IAMRole
	AuthTypeGrafanaAssumeRole //cloud only
)

func (at AuthType) String() string {
	switch at {
	case AuthTypeDefault:
		return "default"
	case AuthTypeSharedCreds:
		return "credentials"
	case AuthTypeKeys:
		return "keys"
	case AuthTypeEC2IAMRole:
		return "ec2_iam_role"
	case AuthTypeGrafanaAssumeRole:
		return "grafana_assume_role"
	default:
		panic(fmt.Sprintf("Unrecognized auth type %d", at))
	}
}

func ToAuthType(authType string) (AuthType, error) {
	switch authType {
	case "credentials", "sharedCreds":
		return AuthTypeSharedCreds, nil
	case "keys":
		return AuthTypeKeys, nil
	case "default":
		return AuthTypeDefault, nil
	case "ec2_iam_role":
		return AuthTypeEC2IAMRole, nil
	case "arn":
		return AuthTypeDefault, nil
	case "grafana_assume_role":
		return AuthTypeGrafanaAssumeRole, nil
	default:
		return AuthTypeDefault, fmt.Errorf("invalid auth type: %s", authType)
	}
}

// MarshalJSON marshals the enum as a quoted json string
func (at *AuthType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(at.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted json string to the enum value
func (at *AuthType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	switch j {
	case "credentials": // Old name
		fallthrough
	case "sharedCreds":
		*at = AuthTypeSharedCreds
	case "keys":
		*at = AuthTypeKeys
	case "ec2_iam_role":
		*at = AuthTypeEC2IAMRole
	case "grafana_assume_role":
		*at = AuthTypeGrafanaAssumeRole
	case "default":
		fallthrough
	default:
		*at = AuthTypeDefault // For old 'arn' option
	}
	return nil
}
