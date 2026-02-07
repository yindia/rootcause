package awsec2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2ListListenerRulesByArn(t *testing.T) {
	responses := map[string]string{
		"DescribeRules": `<DescribeRulesResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeRulesResult>
    <Rules>
      <member>
        <RuleArn>arn:rule</RuleArn>
        <Priority>1</Priority>
        <IsDefault>false</IsDefault>
      </member>
    </Rules>
  </DescribeRulesResult>
</DescribeRulesResponse>`,
	}
	client := newELBTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleListListenerRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"ruleArns": []string{"arn:rule"},
	}}); err != nil {
		t.Fatalf("list listener rules by arn: %v", err)
	}
}
