package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type SpheronApi struct {
	spheronApiUrl string
	token         string

	organizationId string
}

func NewSpheronApi(token string) (*SpheronApi, error) {
	api := &SpheronApi{
		spheronApiUrl: "https://api-v2.spheron.network",
		token:         token,
	}

	return api, nil
}

func (api *SpheronApi) sendApiRequest(method string, path string, payload interface{}, params map[string]interface{}) ([]byte, error) {
	client := &http.Client{Timeout: 600 * time.Second}

	var jsonPayload []byte
	if payload != nil {
		var err error
		jsonPayload, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	request, err := http.NewRequest(method, api.spheronApiUrl+path, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+api.token)

	queryParams := request.URL.Query()
	for key, value := range params {
		queryParams.Add(key, value.(string))
	}
	request.URL.RawQuery = queryParams.Encode()

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		return body, nil
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var errorResponse struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return nil, errors.New("API request failed with status: " + response.Status)
	}

	return nil, errors.New(errorResponse.Message)
}

func (api *SpheronApi) getTokenScope() (TokenScope, error) {
	var tokenScope TokenScope
	path := "/v1/api-keys/scope"

	response, err := api.sendApiRequest(HttpMethodGet, path, nil, nil)
	if err != nil {
		return tokenScope, err
	}

	if err := json.Unmarshal(response, &tokenScope); err != nil {
		return tokenScope, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return tokenScope, nil
}

func (api *SpheronApi) GetOrganizationId() (string, error) {
	if api.organizationId == "" {
		tokenScope, err := api.getTokenScope()
		if err != nil {
			return "", err
		}

		if len(tokenScope.Organizations) != 1 {
			return "", errors.New("Unsupported token! Please use a single scope token.")
		}

		api.organizationId = tokenScope.Organizations[0].ID
	}

	return api.organizationId, nil
}

func (api *SpheronApi) getOrganizationById(id string) (Organization, error) {
	var organization Organization
	response, err := api.sendApiRequest(HttpMethodGet, fmt.Sprintf("/v1/organization/%s", id), nil, nil)

	if err != nil {
		return organization, err
	}

	if err := json.Unmarshal(response, &organization); err != nil {
		return organization, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return organization, nil
}

func (api *SpheronApi) GetOrganization() (Organization, error) {
	organizationId, err := api.GetOrganizationId()
	if err != nil {
		return Organization{}, err
	}

	organization, err := api.getOrganizationById(organizationId)
	if err != nil {
		return Organization{}, err
	}

	return organization, nil
}

func (api *SpheronApi) CreateClusterInstance(clusterInstance CreateInstanceRequest) (InstanceResponse, error) {
	var instanceResponse InstanceResponse
	response, err := api.sendApiRequest(HttpMethodPost, "/v1/cluster-instance/create", clusterInstance, nil)
	if err != nil {
		return instanceResponse, err
	}

	if err := json.Unmarshal(response, &instanceResponse); err != nil {
		return instanceResponse, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return instanceResponse, nil
}

func (api *SpheronApi) CloseClusterInstance(id string) (GenericResponse, error) {
	path := fmt.Sprintf("/v1/cluster-instance/%s/close", id)

	responseBytes, err := api.sendApiRequest("POST", path, nil, nil)
	if err != nil {
		return GenericResponse{}, err
	}

	var response GenericResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return GenericResponse{}, err
	}

	return response, nil
}

func (api *SpheronApi) UpdateClusterInstance(id string, clusterInstance UpdateInstanceRequest) (InstanceResponse, error) {
	path := fmt.Sprintf("/v1/cluster-instance/%s/update", id)

	responseBytes, err := api.sendApiRequest("PATCH", path, clusterInstance, nil)
	if err != nil {
		return InstanceResponse{}, err
	}

	var response InstanceResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return InstanceResponse{}, err
	}

	return response, nil
}

func (api *SpheronApi) UpdateClusterInstanceHealthCheckInfo(id string, healthCheck HealthCheckUpdateReq) (GenericResponse, error) {
	path := fmt.Sprintf("/v1/cluster-instance/%s/update/health-check", id)

	responseBytes, err := api.sendApiRequest("PATCH", path, healthCheck, nil)
	if err != nil {
		return GenericResponse{}, err
	}

	var response GenericResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return GenericResponse{}, err
	}

	return response, nil
}

func (api *SpheronApi) GetClusterInstance(id string) (Instance, error) {
	path := fmt.Sprintf("/v1/cluster-instance/%s", id)

	responseBytes, err := api.sendApiRequest("GET", path, nil, nil)
	if err != nil {
		return Instance{}, err
	}

	var response GetClusterInstanceResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return Instance{}, err
	}

	return response.Instance, nil
}

func (api *SpheronApi) WaitForDeployedEvent(ctx context.Context, topicID string) (string, error) {
	url := fmt.Sprintf(api.spheronApiUrl+"/v1/subscribe?sessionId=%s", topicID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+api.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(line, "event: message") {
			data, err := reader.ReadString('\n')
			tflog.Info(ctx, fmt.Sprintf("%s", data))

			if err != nil {
				return "", err
			}

			if strings.Contains(data, `"type":2`) {
				return data, nil
			}

			if strings.Contains(data, `"type":3`) {
				return "", fmt.Errorf("Deployment failed")
			}
		}
	}
}

func (api *SpheronApi) AddClusterInstanceDomain(instanceID string, domain DomainRequest) (Domain, error) {
	path := fmt.Sprintf("/v1/cluster-instance/%s/domains", instanceID)

	responseBytes, err := api.sendApiRequest("POST", path, domain, nil)
	if err != nil {
		return Domain{}, err
	}

	var response DomainResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return Domain{}, err
	}

	return response.Domain, nil
}

func (api *SpheronApi) UpdateClusterInstanceDomain(instanceID, domainID string, domain DomainRequest) (Domain, error) {
	path := fmt.Sprintf("/v1/cluster-instance/%s/domains/%s", instanceID, domainID)

	responseBytes, err := api.sendApiRequest("PATCH", path, domain, nil)
	if err != nil {
		return Domain{}, err
	}

	var response DomainResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return Domain{}, err
	}

	return response.Domain, nil
}

func (api *SpheronApi) DeleteClusterInstanceDomain(instanceID, domainID string) error {
	path := fmt.Sprintf("/v1/cluster-instance/%s/domains/%s", instanceID, domainID)

	_, err := api.sendApiRequest("DELETE", path, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func (api *SpheronApi) GetClusterInstanceOrder(id string) (InstanceOrder, error) {
	path := fmt.Sprintf("/v1/cluster-instance/order/%s", id)

	responseBytes, err := api.sendApiRequest("GET", path, nil, nil)
	if err != nil {
		return InstanceOrder{}, err
	}

	var response struct {
		Order    InstanceOrder
		LiveLogs []string
	}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return InstanceOrder{}, err
	}

	return response.Order, nil
}

func (api *SpheronApi) CreateClusterInstanceFromTemplate(request CreateInstanceFromMarketplaceRequest) (InstanceResponse, error) {
	path := "/v1/cluster-instance/template"

	responseBytes, err := api.sendApiRequest("POST", path, request, nil)
	if err != nil {
		return InstanceResponse{}, err
	}

	var response InstanceResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return InstanceResponse{}, err
	}

	return response, nil
}

func (api *SpheronApi) GetClusterTemplates() ([]MarketplaceApp, error) {
	path := "/v1/cluster-templates"

	responseBytes, err := api.sendApiRequest("GET", path, nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		ClusterTemplates []MarketplaceApp `json:"clusterTemplates"`
	}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, err
	}

	return response.ClusterTemplates, nil
}

func (api *SpheronApi) GetComputeMachines() ([]ComputeMachine, error) {
	path := "/v1/compute-machine-image"

	requestOptions := map[string]interface{}{
		"skip":  "0",
		"limit": "10",
	}

	responseBytes, err := api.sendApiRequest("GET", path, nil, requestOptions)
	if err != nil {
		return nil, err
	}

	var response struct {
		AkashMachineImages []ComputeMachine `json:"akashMachineImages"`
		TotalCount         int              `json:"totalCount"`
	}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, err
	}

	return response.AkashMachineImages, nil
}

func (api *SpheronApi) GetCluster(id string) (Cluster, error) {
	response, err := api.sendApiRequest(HttpMethodGet, fmt.Sprintf("/v1/cluster/%s", id), nil, nil)
	if err != nil {
		return Cluster{}, err
	}

	var responseWrapper struct {
		Cluster Cluster `json:"cluster"`
	}
	err = json.Unmarshal(response, &responseWrapper)
	if err != nil {
		return Cluster{}, err
	}
	return responseWrapper.Cluster, nil
}

func (api *SpheronApi) GetClusterInstanceDomains(id string) ([]Domain, error) {
	response, err := api.sendApiRequest(HttpMethodGet, fmt.Sprintf("/v1/cluster-instance/%s/domains", id), nil, nil)
	if err != nil {
		return []Domain{}, err
	}

	var responseWrapper struct {
		Domains []Domain `json:"domains"`
	}
	err = json.Unmarshal(response, &responseWrapper)
	if err != nil {
		return []Domain{}, err
	}
	return responseWrapper.Domains, nil
}

const (
	HttpMethodGet    = "GET"
	HttpMethodPost   = "POST"
	HttpMethodPatch  = "PATCH"
	HttpMethodDelete = "DELETE"
	HttpMethodPut    = "PUT"
)
