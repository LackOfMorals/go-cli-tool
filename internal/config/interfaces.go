package config

// Service is the public interface for config access.
type Service interface {
	LoadConfiguration() (Config, error)
}

// configLoader is the internal interface for the Viper-backed loading backend.
// It is not exported; callers interact through Service.
type configLoader interface {
	unmarshal() (Config, error)
}
