package awsec2

func schemaEC2ListInstances() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instanceIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"vpcId":    map[string]any{"type": "string"},
			"subnetId": map[string]any{"type": "string"},
			"state":    map[string]any{"type": "string"},
			"limit":    map[string]any{"type": "number"},
			"region":   map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetInstance() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instanceId": map[string]any{"type": "string"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"instanceId"},
	}
}

func schemaEC2ListASGs() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"autoScalingGroupNames": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetASG() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"autoScalingGroupName": map[string]any{"type": "string"},
			"region":               map[string]any{"type": "string"},
		},
		"required": []string{"autoScalingGroupName"},
	}
}

func schemaEC2ListLoadBalancers() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"loadBalancerArns": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"names": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetLoadBalancer() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"loadBalancerArn": map[string]any{"type": "string"},
			"name":            map[string]any{"type": "string"},
			"region":          map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetTargetHealth() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"targetGroupArn": map[string]any{"type": "string"},
			"targetIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"region": map[string]any{"type": "string"},
		},
		"required": []string{"targetGroupArn"},
	}
}

func schemaEC2ListListenerRules() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"listenerArn": map[string]any{"type": "string"},
			"ruleArns": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetListenerRule() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ruleArn": map[string]any{"type": "string"},
			"region":  map[string]any{"type": "string"},
		},
		"required": []string{"ruleArn"},
	}
}

func schemaEC2ListAutoScalingPolicies() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"autoScalingGroupName": map[string]any{"type": "string"},
			"policyNames": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"policyTypes": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetAutoScalingPolicy() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"autoScalingGroupName": map[string]any{"type": "string"},
			"policyName":           map[string]any{"type": "string"},
			"region":               map[string]any{"type": "string"},
		},
	}
}

func schemaEC2ListScalingActivities() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"autoScalingGroupName": map[string]any{"type": "string"},
			"activityIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetScalingActivity() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"activityId":           map[string]any{"type": "string"},
			"autoScalingGroupName": map[string]any{"type": "string"},
			"region":               map[string]any{"type": "string"},
		},
		"required": []string{"activityId"},
	}
}

func schemaEC2ListLaunchTemplates() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"launchTemplateIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"names": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetLaunchTemplate() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"launchTemplateId": map[string]any{"type": "string"},
			"name":             map[string]any{"type": "string"},
			"region":           map[string]any{"type": "string"},
		},
	}
}

func schemaEC2ListLaunchConfigurations() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"launchConfigurationNames": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetLaunchConfiguration() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"launchConfigurationName": map[string]any{"type": "string"},
			"region":                  map[string]any{"type": "string"},
		},
		"required": []string{"launchConfigurationName"},
	}
}

func schemaEC2GetInstanceIAM() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instanceId": map[string]any{"type": "string"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"instanceId"},
	}
}

func schemaEC2GetSecurityGroupRules() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"groupId": map[string]any{"type": "string"},
			"region":  map[string]any{"type": "string"},
		},
		"required": []string{"groupId"},
	}
}

func schemaEC2ListSpotInstanceRequests() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"spotInstanceRequestIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"states": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetSpotInstanceRequest() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"spotInstanceRequestId": map[string]any{"type": "string"},
			"region":                map[string]any{"type": "string"},
		},
		"required": []string{"spotInstanceRequestId"},
	}
}

func schemaEC2ListCapacityReservations() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"capacityReservationIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetCapacityReservation() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"capacityReservationId": map[string]any{"type": "string"},
			"region":                map[string]any{"type": "string"},
		},
		"required": []string{"capacityReservationId"},
	}
}

func schemaEC2ListVolumes() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"volumeIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"instanceId": map[string]any{"type": "string"},
			"limit":      map[string]any{"type": "number"},
			"region":     map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetVolume() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"volumeId": map[string]any{"type": "string"},
			"region":   map[string]any{"type": "string"},
		},
		"required": []string{"volumeId"},
	}
}

func schemaEC2ListSnapshots() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"snapshotIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"ownerIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"volumeId": map[string]any{"type": "string"},
			"limit":    map[string]any{"type": "number"},
			"region":   map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetSnapshot() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"snapshotId": map[string]any{"type": "string"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"snapshotId"},
	}
}

func schemaEC2ListVolumeAttachments() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"volumeId":   map[string]any{"type": "string"},
			"instanceId": map[string]any{"type": "string"},
			"limit":      map[string]any{"type": "number"},
			"region":     map[string]any{"type": "string"},
		},
	}
}

func schemaEC2ListPlacementGroups() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"groupNames": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetPlacementGroup() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"groupName": map[string]any{"type": "string"},
			"region":    map[string]any{"type": "string"},
		},
		"required": []string{"groupName"},
	}
}

func schemaEC2ListInstanceStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instanceIds": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"includeAll": map[string]any{"type": "boolean"},
			"limit":      map[string]any{"type": "number"},
			"region":     map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetInstanceStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instanceId": map[string]any{"type": "string"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"instanceId"},
	}
}

func schemaEC2ListTargetGroups() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"targetGroupArns": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"names": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"loadBalancerArn": map[string]any{"type": "string"},
			"limit":           map[string]any{"type": "number"},
			"region":          map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetTargetGroup() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"targetGroupArn": map[string]any{"type": "string"},
			"name":           map[string]any{"type": "string"},
			"region":         map[string]any{"type": "string"},
		},
	}
}

func schemaEC2ListListeners() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"listenerArns": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"loadBalancerArn": map[string]any{"type": "string"},
			"limit":           map[string]any{"type": "number"},
			"region":          map[string]any{"type": "string"},
		},
	}
}

func schemaEC2GetListener() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"listenerArn": map[string]any{"type": "string"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"listenerArn"},
	}
}
