package awsvpc

func schemaVPCListVPCs() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetVPC() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId":  map[string]any{"type": "string"},
			"region": map[string]any{"type": "string"},
		},
		"required": []string{"vpcId"},
	}
}

func schemaVPCListSubnets() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"subnetIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetSubnet() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subnetId": map[string]any{"type": "string"},
			"region":   map[string]any{"type": "string"},
		},
		"required": []string{"subnetId"},
	}
}

func schemaVPCListRouteTables() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"routeTableIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetRouteTable() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"routeTableId": map[string]any{"type": "string"},
			"region":       map[string]any{"type": "string"},
		},
		"required": []string{"routeTableId"},
	}
}

func schemaVPCListNatGateways() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"subnetId": map[string]any{
				"type": "string",
			},
			"natGatewayIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetNatGateway() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"natGatewayId": map[string]any{"type": "string"},
			"region":       map[string]any{"type": "string"},
		},
		"required": []string{"natGatewayId"},
	}
}

func schemaVPCListSecurityGroups() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"groupIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetSecurityGroup() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"groupId": map[string]any{"type": "string"},
			"region":  map[string]any{"type": "string"},
		},
		"required": []string{"groupId"},
	}
}

func schemaVPCListNetworkAcls() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"networkAclIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetNetworkAcl() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"networkAclId": map[string]any{"type": "string"},
			"region":       map[string]any{"type": "string"},
		},
		"required": []string{"networkAclId"},
	}
}

func schemaVPCListInternetGateways() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"internetGatewayIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetInternetGateway() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"internetGatewayId": map[string]any{"type": "string"},
			"region":            map[string]any{"type": "string"},
		},
		"required": []string{"internetGatewayId"},
	}
}

func schemaVPCListEndpoints() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId": map[string]any{"type": "string"},
			"endpointIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetEndpoint() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"endpointId": map[string]any{"type": "string"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"endpointId"},
	}
}

func schemaVPCListNetworkInterfaces() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId":    map[string]any{"type": "string"},
			"subnetId": map[string]any{"type": "string"},
			"networkInterfaceIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetNetworkInterface() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"networkInterfaceId": map[string]any{"type": "string"},
			"region":             map[string]any{"type": "string"},
		},
		"required": []string{"networkInterfaceId"},
	}
}

func schemaVPCListResolverEndpoints() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vpcId":  map[string]any{"type": "string"},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetResolverEndpoint() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"resolverEndpointId": map[string]any{"type": "string"},
			"region":             map[string]any{"type": "string"},
		},
		"required": []string{"resolverEndpointId"},
	}
}

func schemaVPCListResolverRules() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"resolverEndpointId": map[string]any{"type": "string"},
			"ruleType":           map[string]any{"type": "string"},
			"limit":              map[string]any{"type": "number"},
			"region":             map[string]any{"type": "string"},
		},
	}
}

func schemaVPCGetResolverRule() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"resolverRuleId": map[string]any{"type": "string"},
			"region":         map[string]any{"type": "string"},
		},
		"required": []string{"resolverRuleId"},
	}
}
