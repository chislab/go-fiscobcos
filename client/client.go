package client

type Client struct {
	Backend
}

func New(cfg *Config) (*Client, error) {
	var backend Backend
	var err error
	if cfg.UseChannel {
		backend, err = newChannel(cfg.CAFile, cfg.CertFile, cfg.KeyFile, cfg.Endpoint, cfg.GroupID)
	} else {
		backend, err = dialRPC(cfg.Endpoint, cfg.GroupID)
	}
	if err != nil {
		return nil, err
	}
	cli := &Client{
		Backend: backend,
	}
	return cli, nil
}
