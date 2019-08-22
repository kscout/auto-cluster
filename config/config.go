package config

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/kscout/auto-cluster/cluster"

	"github.com/Noah-Huppert/goconf"
	"gopkg.in/yaml.v2"
)

// Config allows the user to define the tool's behavior
// NewConfig() must be called to properly initialize struct fields.
type Config struct {

	// Archetypes of clusters to create
	Archetypes []cluster.ArchetypeSpec `mapstructure:"archetypes" validate:"required"`

	// PullSecretPath is the path to a file which contains a
	// pull secret.
	//
	// A pull secret is a Red Hat container registry authentication
	// token. It is used to pull OpenShift container images
	// when creating OpenShift clusters.
	//
	// This field does not have a value when returned
	// by NewConfig(). Instead the PullSecret field is populated.
	PullSecretPath string `mapstructure:"pullSecretPath" validate:"required"`

	// StateDir is the directory which cluster state will be stored
	// within. This directory must persist in order for the tool
	// to properly manage clusters.
	StateDir string `mapstructure:"stateDir" validate:"required"`

	// PullSecret is the contents of the PullSecretPath file
	PullSecret string
}

// NewConfig loads configuration from YAML files in the processes working
// directory or /etc/auto-cluster.
func NewConfig() (Config, error) {
	// Load configuration
	ldr := goconf.NewDefaultLoader()

	ldr.RegisterFormat(".yaml", yamlDecoder{})

	ldr.AddConfigPath("./*.yaml")
	ldr.AddConfigPath("/etc/auto-cluster/*.yaml")

	cfg := Config{}
	if err := ldr.Load(&cfg); err != nil {
		return cfg, err
	}

	// Initialize ArchetypeSpecs
	intdArchs := []cluster.ArchetypeSpec{}
	for _, archetype := range cfg.Archetypes {
		pntrArch := &archetype
		err := pntrArch.Init()
		if err != nil {
			return cfg, fmt.Errorf("failed to initialize archetype with "+
				"name prefix %s: %s", archetype.NamePrefix, err.Error())
		}
		intdArchs = append(intdArchs, *pntrArch)
	}
	cfg.Archetypes = intdArchs

	// Read pull secret
	pullSecBytes, err := ioutil.ReadFile(cfg.PullSecretPath)
	if err != nil {
		return cfg, fmt.Errorf("failed to read pull secret "+
			"file %s: %s", cfg.PullSecretPath, err.Error())
	}
	cfg.PullSecret = string(pullSecBytes)

	return cfg, nil
}

// redact returns a redacted version of a string providing enough debug
// information to learn about a field without exposing its value
func redact(in string) string {
	if len(in) > 0 {
		return "REDACTED_NOT_EMPTY"
	}

	return ""
}

// String returns a string representation of the Config.
// This should always be used when printing Config to avoid exposing
// secure values.
func (c Config) String() string {
	c.PullSecret = redact(c.PullSecret)

	return fmt.Sprintf("%#v", c)
}

// yamlDecoder decodes YAML files into config
type yamlDecoder struct{}

// Decoder implements github.com/Noah-Huppert/goconf.MapDecoder.Decode
func (d yamlDecoder) Decode(r io.Reader, m *map[string]interface{}) error {
	decoder := yaml.NewDecoder(r)
	return decoder.Decode(m)
}
