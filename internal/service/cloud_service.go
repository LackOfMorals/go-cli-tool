package service

import (
	"context"
	"fmt"

	"github.com/cli/go-cli-tool/internal/config"
)

// CloudServiceImpl is the top-level Aura cloud service.
type CloudServiceImpl struct {
	cfg       config.AuraConfig
	instances InstancesService
	projects  ProjectsService
}

// NewCloudService creates a CloudService using the provided Aura API config.
// When ClientID or ClientSecret are empty the sub-services return a clear
// error message rather than panicking — replace the stubs with real Aura
// API client calls once the aura-api-client library is wired in.
func NewCloudService(cfg config.AuraConfig) CloudService {
	return &CloudServiceImpl{
		cfg:       cfg,
		instances: &instancesServiceStub{cfg: cfg},
		projects:  &projectsServiceStub{cfg: cfg},
	}
}

func (s *CloudServiceImpl) Instances() InstancesService { return s.instances }
func (s *CloudServiceImpl) Projects() ProjectsService   { return s.projects }

// ---- shared credential check --------------------------------------------

func checkAuraCreds(cfg config.AuraConfig) error {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return fmt.Errorf(
			"Aura API credentials not configured — " +
				"set CLI_AURA_CLIENT_ID and CLI_AURA_CLIENT_SECRET " +
				"(or aura.client_id / aura.client_secret in your config file)",
		)
	}
	return nil
}

// ---- Instances stub -----------------------------------------------------

type instancesServiceStub struct{ cfg config.AuraConfig }

func (s *instancesServiceStub) List(_ context.Context) ([]Instance, error) {
	return nil, checkAuraCreds(s.cfg)
}

func (s *instancesServiceStub) Get(_ context.Context, id string) (*Instance, error) {
	if err := checkAuraCreds(s.cfg); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("instances.Get(%s): not yet implemented", id)
}

func (s *instancesServiceStub) Pause(_ context.Context, id string) error {
	if err := checkAuraCreds(s.cfg); err != nil {
		return err
	}
	return fmt.Errorf("instances.Pause(%s): not yet implemented", id)
}

func (s *instancesServiceStub) Resume(_ context.Context, id string) error {
	if err := checkAuraCreds(s.cfg); err != nil {
		return err
	}
	return fmt.Errorf("instances.Resume(%s): not yet implemented", id)
}

func (s *instancesServiceStub) Delete(_ context.Context, id string) error {
	if err := checkAuraCreds(s.cfg); err != nil {
		return err
	}
	return fmt.Errorf("instances.Delete(%s): not yet implemented", id)
}

// ---- Projects stub ------------------------------------------------------

type projectsServiceStub struct{ cfg config.AuraConfig }

func (s *projectsServiceStub) List(_ context.Context) ([]Project, error) {
	return nil, checkAuraCreds(s.cfg)
}

func (s *projectsServiceStub) Get(_ context.Context, id string) (*Project, error) {
	if err := checkAuraCreds(s.cfg); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("projects.Get(%s): not yet implemented", id)
}
