package libp2pquic

type Option func(opts *Config) error

type Config struct {
	disableReuseport bool
}

func (cfg *Config) apply(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return err
		}
	}

	return nil
}

func DisableReuseport() Option {
	return func(cfg *Config) error {
		cfg.disableReuseport = true
		return nil
	}
}
