package awsec2

import (
	"encoding/json"
	"testing"
)

func TestEC2TypeHelpers(t *testing.T) {
	if got := toInt(json.Number("7"), 1); got != 7 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := toBool(nil, true); got != true {
		t.Fatalf("unexpected toBool fallback: %v", got)
	}
	if got := instanceProfileNameFromArn("arn:aws:iam::123:instance-profile/team/profile"); got != "profile" {
		t.Fatalf("unexpected instance profile name: %s", got)
	}
	if got := instanceProfileNameFromArn("bad-arn"); got != "" {
		t.Fatalf("expected empty instance profile name")
	}
}
