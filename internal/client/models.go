package client

import (
	"time"
)

type TokenScope struct {
	User          User                `json:"user"`
	Organizations []TokenOrganization `json:"organizations"`
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

type TokenOrganization struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type Organization struct {
	ID      string `json:"_id"`
	Profile struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Image    string `json:"image"`
	} `json:"profile"`
}

type CreateInstanceRequest struct {
	OrganizationID  string                `json:"organizationId"`
	UniqueTopicID   string                `json:"uniqueTopicId"`
	Configuration   InstanceConfiguration `json:"configuration"`
	InstanceName    string                `json:"instanceName,omitempty"`
	ClusterURL      string                `json:"clusterUrl"`
	ClusterProvider string                `json:"clusterProvider"`
	ClusterName     string                `json:"clusterName"`
	HealthCheckURL  string                `json:"healthCheckUrl"`
	HealthCheckPort string                `json:"healthCheckPort"`
}

type InstanceConfiguration struct {
	Branch                string              `json:"branch"`
	FolderName            string              `json:"folderName"`
	Protocol              ClusterProtocolEnum `json:"protocol"`
	Image                 string              `json:"image"`
	Tag                   string              `json:"tag"`
	InstanceCount         int                 `json:"instanceCount"`
	BuildImage            bool                `json:"buildImage"`
	Ports                 []Port              `json:"ports"`
	Env                   []Env               `json:"env"`
	Command               []string            `json:"command"`
	Args                  []string            `json:"args"`
	Region                string              `json:"region"`
	AkashMachineImageName string              `json:"akashMachineImageName"`
	CustomInstanceSpecs   CustomInstanceSpecs `json:"customInstanceSpecs"`
}

type CustomInstanceSpecs struct {
	CPU               string            `json:"cpu,omitempty"`
	Memory            string            `json:"memory,omitempty"`
	PersistentStorage PersistentStorage `json:"persistentStorage,omitempty"`
	Storage           string            `json:"storage"`
}

type PersistentStorage struct {
	Class      string `json:"class,omitempty"`
	MountPoint string `json:"mountPoint,omitempty"`
	Size       string `json:"size,omitempty"`
}

type UpdateInstanceRequest struct {
	Env            []Env    `json:"env"`
	Command        []string `json:"command"`
	Args           []string `json:"args"`
	UniqueTopicID  string   `json:"uniqueTopicId"`
	Tag            string   `json:"tag"`
	OrganizationID string   `json:"organizationId"`
}

type HealthCheckUpdateReq struct {
	HealthCheckURL  string `json:"healthCheckUrl"`
	HealthCheckPort int    `json:"healthCheckPort"`
}

type Port struct {
	ContainerPort int `json:"containerPort"`
	ExposedPort   int `json:"exposedPort"`
}

type Env struct {
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
}

type ClusterProtocolEnum string

const (
	ClusterProtocolAkash ClusterProtocolEnum = "akash"
)

type InstanceResponse struct {
	ClusterID              string `json:"clusterId"`
	ClusterInstanceID      string `json:"clusterInstanceId"`
	ClusterInstanceOrderID string `json:"clusterInstanceOrderId"`
	Topic                  string `json:"topic"`
}

type GenericResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success,omitempty"`
	Updated bool   `json:"updated,omitempty"`
}

type GetClusterInstanceResponse struct {
	Success  bool     `json:"success"`
	Instance Instance `json:"instance"`
}

type Instance struct {
	ID                     string           `json:"_id"`
	State                  string           `json:"state"`
	Name                   string           `json:"name"`
	Orders                 []string         `json:"orders"`
	Cluster                string           `json:"cluster"`
	ActiveOrder            string           `json:"activeOrder"`
	LatestURLPreview       string           `json:"latestUrlPreview"`
	AgreedMachineImageType MachineImageType `json:"agreedMachineImageType"`
	RetrievableAkt         int              `json:"retrievableAkt"`
	WithdrawnAkt           int              `json:"withdrawnAkt"`
	HealthCheck            HealthCheck      `json:"healthCheck"`
	CreatedAt              time.Time        `json:"createdAt"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

type MachineImageType struct {
	MachineType       string             `json:"machineType"`
	Storage           string             `json:"storage"`
	Cpu               float32            `json:"cpu"`
	Memory            string             `json:"memory"`
	PersistentStorage *PersistentStorage `json:"persistentStorage,omitempty"`
}

type HealthCheck struct {
	URL       string    `json:"url"`
	Port      Port      `json:"port,omitempty"`
	Status    string    `json:"status,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

type DomainRequest struct {
	Link string         `json:"link"`
	Type DomainTypeEnum `json:"type"`
	Name string         `json:"name"`
}

type DomainResponse struct {
	Domain Domain `json:"domain"`
}

type Domain struct {
	ID         string         `json:"_id"`
	Name       string         `json:"name"`
	Verified   bool           `json:"verified"`
	Link       string         `json:"link"`
	Type       DomainTypeEnum `json:"type"`
	InstanceID string         `json:"instanceId"`
}

type DomainTypeEnum string

const (
	DomainTypeDomain    DomainTypeEnum = "domain"
	DomainTypeSubdomain DomainTypeEnum = "subdomain"
)

type InstanceOrder struct {
	ID                           string                        `json:"_id"`
	Status                       string                        `json:"status"`
	Env                          map[string]interface{}        `json:"env"`
	URLPreview                   string                        `json:"urlPrewiew"`
	ProtocolData                 *ProtocolData                 `json:"protocolData,omitempty"`
	ClusterInstanceConfiguration *ClusterInstanceConfiguration `json:"clusterInstanceConfiguration,omitempty"`
}

type ProtocolData struct {
	ProviderHost string `json:"providerHost"`
}

type ClusterInstanceConfiguration struct {
	Image              string           `json:"image"`
	Tag                string           `json:"tag"`
	Ports              []Port           `json:"ports"`
	Env                []Env            `json:"env"`
	Command            []string         `json:"command"`
	Args               []string         `json:"args"`
	Region             string           `json:"region"`
	AgreedMachineImage MachineImageType `json:"agreedMachineImage"`
	InstanceCount      int              `json:"instanceCount"`
}

type MarketplaceApp struct {
	ID          string                    `json:"_id"`
	Name        string                    `json:"name"`
	ServiceData MarketplaceAppServiceData `json:"serviceData"`
}

type MarketplaceAppServiceData struct {
	Variables []MarketplaceAppVariable `json:"variables"`
}

type MarketplaceAppVariable struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	DefaultValue string `json:"defaultValue,omitempty"`
	Required     bool   `json:"required,omitempty"`
}

type CreateInstanceFromMarketplaceRequest struct {
	TemplateID           string                          `json:"templateId"`
	EnvironmentVariables []MarketplaceDeploymentVariable `json:"environmentVariables"`
	OrganizationID       string                          `json:"organizationId"`
	AkashImageID         string                          `json:"akashImageId"`
	UniqueTopicID        string                          `json:"uniqueTopicId,omitempty"`
	Region               string                          `json:"region"`
	CustomInstanceSpecs  CustomInstanceSpecs             `json:"customInstanceSpecs"`
	InstanceCount        int                             `json:"instanceCount"`
}

type MarketplaceDeploymentVariable struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ComputeMachine struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type Cluster struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}
