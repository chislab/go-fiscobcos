package client

type Config struct {
	CAFile     string
	CertFile   string
	KeyFile    string
	Endpoint   string
	UseChannel bool
}
