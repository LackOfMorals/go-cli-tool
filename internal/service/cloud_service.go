package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	aura "github.com/LackOfMorals/aura-client"
	"github.com/cli/go-cli-tool/internal/config"
)

// defaultAuraTimeout is used when AuraConfig.TimeoutSeconds is zero or negative.
const defaultAuraTimeout = 30 * time.Second

// ---- CloudServiceImpl ---------------------------------------------------

// CloudServiceImpl is the top-level Aura cloud service. It holds a pointer to
// AuraConfig so that credentials set interactively after startup (by the
// AuraPrerequisite) are visible to ensureClient when it first runs.
type CloudServiceImpl struct {
	cfg    *config.AuraConfig
	mu     sync.Mutex // guards lazy client initialisation
	client *aura.AuraAPIClient
}

// NewCloudService creates a CloudService backed by the Aura API.
//
// cfg must be a pointer to the live AuraConfig so that credentials written
// interactively by the prerequisite are picked up on the first real call.
// The underlying aura-client is initialised lazily on the first operation,
// so startup is fast even when credentials are not yet configured.
func NewCloudService(cfg *config.AuraConfig) CloudService {
	return &CloudServiceImpl{cfg: cfg}
}

// ensureClient initialises the aura-client on the first call and reuses it
// afterwards. If initialisation fails the error is returned to the caller —
// the next call will try again, which allows the user to correct credentials
// (e.g. via the interactive prerequisite) and retry.
func (s *CloudServiceImpl) ensureClient() (*aura.AuraAPIClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client, nil
	}

	if s.cfg.ClientID == "" || s.cfg.ClientSecret == "" {
		return nil, fmt.Errorf(
			"aura API credentials not configured — " +
				"set CLI_AURA_CLIENT_ID and CLI_AURA_CLIENT_SECRET " +
				"or aura.client_id / aura.client_secret in your config file",
		)
	}

	timeout := time.Duration(s.cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultAuraTimeout
	}

	client, err := aura.NewClient(
		aura.WithCredentials(s.cfg.ClientID, s.cfg.ClientSecret),
		aura.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("initialise Aura client: %w", err)
	}

	s.client = client
	return client, nil
}

// Instances returns an InstancesService. The real API client is created
// lazily on the first operation so that category build-time wiring works
// even before credentials are available.
func (s *CloudServiceImpl) Instances() InstancesService {
	return &instancesServiceImpl{parent: s}
}

// Projects returns a ProjectsService backed by the Aura Tenants API.
func (s *CloudServiceImpl) Projects() ProjectsService {
	return &projectsServiceImpl{parent: s}
}

// ---- instancesServiceImpl -----------------------------------------------

type instancesServiceImpl struct{ parent *CloudServiceImpl }

func (s *instancesServiceImpl) List(ctx context.Context) ([]Instance, error) {
	client, err := s.parent.ensureClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Instances.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}

	instances := make([]Instance, 0, len(resp.Data))
	for _, d := range resp.Data {
		instances = append(instances, Instance{
			ID:            d.ID,
			Name:          d.Name,
			CloudProvider: d.CloudProvider,
			TenantID:      d.TenantID,
		})
	}
	return instances, nil
}

func (s *instancesServiceImpl) Get(ctx context.Context, id string) (*Instance, error) {
	client, err := s.parent.ensureClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Instances.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get instance %s: %w", id, err)
	}

	return instanceFromData(resp.Data), nil
}

func (s *instancesServiceImpl) Create(ctx context.Context, params *CreateInstanceParams) (*CreatedInstance, error) {
	client, err := s.parent.ensureClient()
	if err != nil {
		return nil, err
	}

	cfg := &aura.CreateInstanceConfigData{
		Name:          params.Name,
		TenantID:      params.TenantID,
		CloudProvider: params.CloudProvider,
		Region:        params.Region,
		Type:          params.Type,
		Version:       params.Version,
		Memory:        params.Memory,
	}

	resp, err := client.Instances.Create(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}

	// CreateInstanceData is a distinct type from InstanceData.
	// It has Username and Password but lacks Status and Memory.
	d := resp.Data
	return &CreatedInstance{
		Instance: Instance{
			ID:            d.ID,
			Name:          d.Name,
			Region:        d.Region,
			Tier:          d.Type,
			CloudProvider: d.CloudProvider,
			TenantID:      d.TenantID,
			ConnectionURL: d.ConnectionURL,
			Username:      d.Username,
		},
		Password: d.Password,
	}, nil
}

func (s *instancesServiceImpl) Update(ctx context.Context, id string, params *UpdateInstanceParams) (*Instance, error) {
	client, err := s.parent.ensureClient()
	if err != nil {
		return nil, err
	}

	req := &aura.UpdateInstanceData{
		Name:   params.Name,
		Memory: params.Memory,
	}

	resp, err := client.Instances.Update(ctx, id, req)
	if err != nil {
		return nil, fmt.Errorf("update instance %s: %w", id, err)
	}

	return instanceFromData(resp.Data), nil
}

func (s *instancesServiceImpl) Pause(ctx context.Context, id string) error {
	client, err := s.parent.ensureClient()
	if err != nil {
		return err
	}

	if _, err := client.Instances.Pause(ctx, id); err != nil {
		return fmt.Errorf("pause instance %s: %w", id, err)
	}
	return nil
}

func (s *instancesServiceImpl) Resume(ctx context.Context, id string) error {
	client, err := s.parent.ensureClient()
	if err != nil {
		return err
	}

	if _, err := client.Instances.Resume(ctx, id); err != nil {
		return fmt.Errorf("resume instance %s: %w", id, err)
	}
	return nil
}

func (s *instancesServiceImpl) Delete(ctx context.Context, id string) error {
	client, err := s.parent.ensureClient()
	if err != nil {
		return err
	}

	if _, err := client.Instances.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete instance %s: %w", id, err)
	}
	return nil
}

// instanceFromData maps an aura-client InstanceData to our service type.
// InstanceData.Status is aura.InstanceStatus (a string typedef), so it is
// cast to plain string. InstanceData has no Username field — that is only
// present on CreateInstanceData and is handled separately in Create.
func instanceFromData(d aura.InstanceData) *Instance {
	return &Instance{
		ID:            d.ID,
		Name:          d.Name,
		Status:        string(d.Status),
		Region:        d.Region,
		Tier:          d.Type,
		CloudProvider: d.CloudProvider,
		TenantID:      d.TenantID,
		ConnectionURL: d.ConnectionURL,
		Memory:        d.Memory,
	}
}

// ---- projectsServiceImpl ------------------------------------------------

type projectsServiceImpl struct{ parent *CloudServiceImpl }

func (s *projectsServiceImpl) List(ctx context.Context) ([]Project, error) {
	client, err := s.parent.ensureClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Tenants.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects := make([]Project, 0, len(resp.Data))
	for _, t := range resp.Data {
		projects = append(projects, Project{ID: t.ID, Name: t.Name})
	}
	return projects, nil
}

func (s *projectsServiceImpl) Get(ctx context.Context, id string) (*Project, error) {
	client, err := s.parent.ensureClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Tenants.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get project %s: %w", id, err)
	}

	return &Project{ID: resp.Data.ID, Name: resp.Data.Name}, nil
}
