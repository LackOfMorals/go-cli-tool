package config

// ---- Interfaces ---------------------------------------------------------

// Service is the public interface for config access.
type Service interface {
	LoadConfiguration() (Config, error)
	SaveConfiguration(config Config) error
}

// configLoader is the internal interface for the loading backend.
type configLoader interface {
	load(path string) (Config, error)
	save(path string, config Config) error
	loadFromEnv() Config
}
