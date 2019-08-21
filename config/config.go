package config

import (
	"fmt"
	"io"

	"github.com/kscout/auto-cluster/cluster"

	"github.com/Noah-Huppert/goconf"
	"gopkg.in/yaml.v2"
)

// Config allows the user to define the tool's behavior
type Config struct {
	// Archetypes of clusters to create
	Archetypes []cluster.ArchetypeSpec `mapstructure:"archetypes" validate:"required"`
}

// NewConfig loads configuration from YAML files in the processes working
// directory or /etc/auto-cluster.
func NewConfig() (Config, error) {
	ldr := goconf.NewDefaultLoader()

	ldr.RegisterFormat(".yaml", yamlDecoder{})

	ldr.AddConfigPath("./*.yaml")
	ldr.AddConfigPath("/etc/auto-cluster/*.yaml")

	cfg := Config{}
	if err := ldr.Load(&cfg); err != nil {
		return cfg, err
	}

	intdCfg := Config{}
	for _, archetype := range cfg.Archetypes {
		pntrArch := &archetype
		err := pntrArch.Init()
		if err != nil {
			return cfg, fmt.Errorf("failed to initialize archetype with "+
				"name prefix %s: %s", archetype.NamePrefix, err.Error())
		}
		intdCfg.Archetypes = append(intdCfg.Archetypes, *pntrArch)
	}

	return intdCfg, nil
}

// yamlDecoder decodes YAML files into config
type yamlDecoder struct{}

// Decoder implements github.com/Noah-Huppert/goconf.MapDecoder.Decode
func (d yamlDecoder) Decode(r io.Reader, m *map[string]interface{}) error {
	decoder := yaml.NewDecoder(r)
	return decoder.Decode(m)
}
