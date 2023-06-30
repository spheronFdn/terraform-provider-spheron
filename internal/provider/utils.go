package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"terraform-provider-spheron/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func findComputeMachineID(machines []client.ComputeMachine, name string) (string, error) {
	for _, machine := range machines {
		if machine.Name == name {
			return machine.ID, nil
		}
	}

	return "", errors.New("ComputeMachine not found with the provided ID")
}

func findMarketplaceAppByName(apps []client.MarketplaceApp, name string) (client.MarketplaceApp, error) {
	for _, app := range apps {
		if app.Name == name {
			return app, nil
		}
	}

	return client.MarketplaceApp{}, fmt.Errorf("MarketplaceApp not found with name: %s", name)
}

func mapMarketplaceEnvs(envList []Env) []client.MarketplaceDeploymentVariable {
	marketplaceEnvs := make([]client.MarketplaceDeploymentVariable, 0, len(envList))
	for _, env := range envList {
		marketplaceEnv := client.MarketplaceDeploymentVariable{
			Value: env.Value.ValueString(),
			Label: env.Key.ValueString(),
		}
		marketplaceEnvs = append(marketplaceEnvs, marketplaceEnv)
	}
	return marketplaceEnvs
}

func checkRequiredDeploymentVariables(appVariables []client.MarketplaceAppVariable, envList []Env) ([]client.MarketplaceDeploymentVariable, error) {
	allVariables := make(map[string]client.MarketplaceAppVariable)
	missingVariables := make(map[string]bool)
	deploymentVariables := make([]client.MarketplaceDeploymentVariable, 0, len(envList))

	for _, appVar := range appVariables {
		allVariables[appVar.Name] = appVar
		missingVariables[appVar.Name] = true
	}

	for _, env := range envList {
		if appVar, ok := allVariables[env.Key.ValueString()]; ok {
			marketplaceEnv := client.MarketplaceDeploymentVariable{
				Value: env.Value.ValueString(),
				Label: appVar.Label,
			}
			deploymentVariables = append(deploymentVariables, marketplaceEnv)

			missingVariables[appVar.Name] = false
		}
	}

	for varName, isMissing := range missingVariables {
		if isMissing {
			return nil, errors.New(fmt.Sprintf("Missing required deployment variable: %s", varName))
		}
	}

	return deploymentVariables, nil
}

func mapModelPortToPortValue(portList []client.Port) []attr.Value {
	ports := make([]attr.Value, len(portList))
	for i, pm := range portList {
		portTypes := make(map[string]attr.Type)
		portValues := make(map[string]attr.Value)

		portTypes["container_port"] = types.Int64Type
		portTypes["exposed_port"] = types.Int64Type

		portValues["container_port"] = types.Int64Value(int64(pm.ContainerPort))
		portValues["exposed_port"] = types.Int64Value(int64(pm.ExposedPort))
		port := types.ObjectValueMust(portTypes, portValues)

		ports[i] = port
	}
	return ports
}

func getPortAtrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"container_port": types.Int64Type,
		"exposed_port":   types.Int64Type,
	}
}

func mapClientEnvsToEnvsValue(clientEnvs []client.Env, isSecret bool) []attr.Value {
	if len(clientEnvs) == 0 {
		return nil
	}

	envList := make([]attr.Value, 0, len(clientEnvs))

	for _, clientEnv := range clientEnvs {
		if clientEnv.IsSecret != isSecret {
			continue
		}

		split := strings.SplitN(clientEnv.Value, "=", 2)
		keyString, valueString := split[0], split[1]

		portTypes := make(map[string]attr.Type)
		portValues := make(map[string]attr.Value)

		portTypes["key"] = types.StringType
		portTypes["value"] = types.StringType

		portValues["key"] = types.StringValue(keyString)
		portValues["value"] = types.StringValue(valueString)

		envList = append(envList, types.ObjectValueMust(portTypes, portValues))
	}
	return envList
}

func getEnvAtrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
}

func mapPortToPortModel(portList []Port) []client.Port {
	ports := []client.Port{}
	for _, pm := range portList {
		exposedPort := int(pm.ContainerPort.ValueInt64())
		if pm.ExposedPort.ValueInt64() != 0 {
			exposedPort = int(pm.ExposedPort.ValueInt64())
		}

		port := client.Port{
			ContainerPort: int(pm.ContainerPort.ValueInt64()),
			ExposedPort:   exposedPort,
		}
		ports = append(ports, port)
	}
	return ports
}

func mapModelPortToPort(portList []client.Port) []Port {
	ports := []Port{}
	for _, pm := range portList {
		port := Port{
			ContainerPort: types.Int64Value(int64(pm.ContainerPort)),
			ExposedPort:   types.Int64Value(int64(pm.ExposedPort)),
		}
		ports = append(ports, port)
	}
	return ports
}

func mapEnvsToClientEnvs(envList []Env, isSecret bool) []client.Env {
	clientEnvs := make([]client.Env, 0, len(envList))
	for _, env := range envList {
		clientEnv := client.Env{
			Value:    env.Key.ValueString() + "=" + env.Value.ValueString(),
			IsSecret: isSecret,
		}
		clientEnvs = append(clientEnvs, clientEnv)
	}
	return clientEnvs
}

func mapClientEnvsToEnvs(clientEnvs []client.Env, isSecret bool) []Env {
	envList := make([]Env, 0, len(clientEnvs))

	for _, clientEnv := range clientEnvs {
		if clientEnv.IsSecret != isSecret {
			continue
		}

		split := strings.SplitN(clientEnv.Value, "=", 2)
		keyString, valueString := split[0], split[1]

		newEnv := Env{
			Key:   types.StringValue(keyString),
			Value: types.StringValue(valueString),
		}

		envList = append(envList, newEnv)
	}

	if len(envList) == 0 {
		return nil
	}

	return envList
}

func ParseClientPorts(responseString string) ([]client.Port, error) {
	trimmedString := strings.TrimPrefix(responseString, "data: ")

	type ResponseData struct {
		Type int `json:"type"`
		Data struct {
			DeploymentStatus string        `json:"deploymentStatus"`
			LatestUrlPreview string        `json:"latestUrlPreview"`
			ProviderHost     string        `json:"providerHost"`
			Ports            []client.Port `json:"ports"`
		} `json:"data"`
		Session string `json:"session"`
	}

	var responseData ResponseData
	err := json.Unmarshal([]byte(trimmedString), &responseData)
	if err != nil {
		return nil, err
	}

	return responseData.Data.Ports, nil
}

func isValidDomainType(value string) bool {
	switch client.DomainTypeEnum(value) {
	case client.DomainTypeDomain, client.DomainTypeSubdomain:
		return true
	}
	return false
}

func getPortFromDeploymentURL(input client.InstanceOrder, urlStr string) (int, error) {
	if (input.ProtocolData != nil && input.ProtocolData.ProviderHost != "") || input.URLPreview != "" {
		for _, port := range input.ClusterInstanceConfiguration.Ports {
			if urlStr == input.URLPreview && port.ExposedPort == 80 {
				return port.ContainerPort, nil
			}

			expectedURL := fmt.Sprintf("%s:%d", input.ProtocolData.ProviderHost, port.ExposedPort)
			if urlStr == expectedURL {
				return port.ContainerPort, nil
			}
		}
	}

	return 0, fmt.Errorf("no matching port found for the provided URL")
}

func findDomainByID(domains []client.Domain, id string) (client.Domain, error) {
	for _, domain := range domains {
		if domain.ID == id {
			return domain, nil
		}
	}
	return client.Domain{}, fmt.Errorf("Domain with ID %s not found", id)
}

func getInstanceDeploymentURL(input client.InstanceOrder, desiredPort int) string {
	if (input.ProtocolData != nil && input.ProtocolData.ProviderHost != "") || input.URLPreview != "" {
		for _, port := range input.ClusterInstanceConfiguration.Ports {
			if port.ContainerPort == desiredPort {
				if port.ExposedPort == 80 && input.URLPreview != "" {
					return input.URLPreview
				}

				return fmt.Sprintf("%s:%d", input.ProtocolData.ProviderHost, port.ExposedPort)
			}
		}
	}

	return ""
}

var persistentStorageClassMap = map[string]string{
	"HDD":  "beta1",
	"SSD":  "beta2",
	"NVMe": "beta3",
}

func GetPersistentStorageClassEnum(key string) (string, error) {
	value, ok := persistentStorageClassMap[key]
	if !ok {
		return "", fmt.Errorf("Storage class: %s is not supported. Supported values are: HDD, SSD and NVMe.", value)
	}
	return value, nil
}

func GetStorageClassFromValue(value string) (string, error) {
	reverseMap := make(map[string]string)
	for key, val := range persistentStorageClassMap {
		reverseMap[val] = key
	}

	class, ok := reverseMap[value]
	if !ok {
		return "", fmt.Errorf("value '%s' is not a valid storage class", value)
	}
	return class, nil
}

func RemoveGiSuffix(input string) string {
	if len(input) < 2 {
		return input
	}
	return input[:len(input)-2]
}
