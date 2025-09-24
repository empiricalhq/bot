package logger

type Config struct {
	Level Level
}

type Factory struct {
	config Config
}

func NewFactory(config Config) (*Factory, error) {
	return &Factory{config: config}, nil
}

func (f *Factory) GetLogger(name string) *Logger {
	return NewLogger(name, f.config.Level)
}

func (f *Factory) Close() error {
	return nil // no resources to clean up in Lambda
}
