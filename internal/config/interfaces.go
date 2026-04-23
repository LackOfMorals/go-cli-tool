package config

// Service is the public interface for config access.
type Service interface {
	LoadConfiguration() (Config, error)
	SaveConfiguration(cfg Config) error
}

// configLoader is the internal interface for the Viper-backed loading backend.
// It is not exported; callers interact through Service.
type configLoader interface {
	readFile(path string) error
	unmarshal() (Config, error)
	save(path string, cfg Config) error
}
