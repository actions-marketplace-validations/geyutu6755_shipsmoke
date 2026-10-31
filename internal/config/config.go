// Package config defines the .shipsmoke.yml schema and provides strict
// loading and validation.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root of .shipsmoke.yml.
type Config struct {
	Version   int        `yaml:"version"`
	Safety    Safety     `yaml:"safety,omitempty"`
	Report    ReportCfg  `yaml:"report,omitempty"`
	Artifacts []Artifact `yaml:"artifacts"`
}

// Safety holds extraction limits. Zero values are replaced by defaults in
// ApplyDefaults.
type Safety struct {
	MaxFiles            int   `yaml:"max_files,omitempty"`
	MaxTotalSize        int64 `yaml:"max_total_size,omitempty"`
	MaxFileSize         int64 `yaml:"max_file_size,omitempty"`
	MaxCompressionRatio int64 `yaml:"max_compression_ratio,omitempty"`
}

// ReportCfg configures report outputs.
type ReportCfg struct {
	// JSON is the path where the machine-readable JSON report is written.
	JSON string `yaml:"json,omitempty"`
}

// Artifact describes one release artifact to verify.
type Artifact struct {
	Name   string `yaml:"name,omitempty"`
	Path   string `yaml:"path"`
	Format string `yaml:"format,omitempty"` // auto | zip | tar.gz | binary
	Binary string `yaml:"binary"`
	// RunsOn lists os/arch pairs this artifact targets, e.g. "linux/amd64",
	// or the special value "any". Empty means "any": the artifact is
	// attempted on the current host and platform mismatches are diagnosed.
	RunsOn   []string          `yaml:"runs_on,omitempty"`
	Env      string            `yaml:"env,omitempty"` // minimal | inherit
	EnvExtra map[string]string `yaml:"env_extra,omitempty"`
	Checks   []Check           `yaml:"checks"`
}

// Check is one smoke check: run the entry binary with Args and assert Expect.
type Check struct {
	Args    []string `yaml:"args"`
	Expect  Expect   `yaml:"expect,omitempty"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// Expect holds the assertions applied to one check run. The zero value means
// "expect exit code 0, no output constraints".
type Expect struct {
	ExitCode          int      `yaml:"exit_code"`
	StdoutContains    []string `yaml:"stdout_contains,omitempty"`
	StdoutNotContains []string `yaml:"stdout_not_contains,omitempty"`
	StderrContains    []string `yaml:"stderr_contains,omitempty"`
	StderrNotContains []string `yaml:"stderr_not_contains,omitempty"`
	StdoutEquals      string   `yaml:"stdout_equals,omitempty"`
	StderrEquals      string   `yaml:"stderr_equals,omitempty"`
	StdoutRegex       string   `yaml:"stdout_regex,omitempty"`
	StderrRegex       string   `yaml:"stderr_regex,omitempty"`
}

// Duration wraps time.Duration for YAML "30s" style values.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return fmt.Errorf("timeout must be a string like \"30s\" or \"2m\"")
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: use forms like 30s, 2m, 1m30s", s)
	}
	if dur <= 0 {
		return fmt.Errorf("duration must be positive, got %q", s)
	}
	if dur > maxTimeout {
		return fmt.Errorf("duration %s exceeds the maximum allowed (%s)", dur, maxTimeout)
	}
	d.Duration = dur
	return nil
}

// MarshalYAML renders the duration back in its string form.
func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

const maxTimeout = time.Hour

// Defaults applied when the user leaves values out.
const (
	DefaultMaxFiles            = 20000
	DefaultMaxTotalSize        = int64(1) << 30 // 1 GiB
	DefaultMaxFileSize         = int64(1) << 29 // 512 MiB
	DefaultMaxCompressionRatio = 1000
	DefaultTimeout             = 60 * time.Second
	DefaultFormat              = "auto"
	DefaultEnv                 = "minimal"
)

// DefaultConfigName is the file shipsmoke looks for.
const DefaultConfigName = ".shipsmoke.yml"

// ApplyDefaults fills zero values with documented defaults.
func (c *Config) ApplyDefaults() {
	if c.Safety.MaxFiles == 0 {
		c.Safety.MaxFiles = DefaultMaxFiles
	}
	if c.Safety.MaxTotalSize == 0 {
		c.Safety.MaxTotalSize = DefaultMaxTotalSize
	}
	if c.Safety.MaxFileSize == 0 {
		c.Safety.MaxFileSize = DefaultMaxFileSize
	}
	if c.Safety.MaxCompressionRatio == 0 {
		c.Safety.MaxCompressionRatio = DefaultMaxCompressionRatio
	}
	for i := range c.Artifacts {
		a := &c.Artifacts[i]
		if a.Format == "" {
			a.Format = DefaultFormat
		}
		if a.Env == "" {
			a.Env = DefaultEnv
		}
		if a.Name == "" {
			a.Name = baseName(a.Path)
		}
		for j := range a.Checks {
			if a.Checks[j].Timeout.Duration == 0 {
				a.Checks[j].Timeout.Duration = DefaultTimeout
			}
		}
	}
}

// Load reads, strictly decodes (unknown keys are errors), applies defaults and
// validates the config at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config: %w", err)
	}
	return Parse(data)
}

// Parse is Load from raw bytes.
func Parse(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(bytesReader(data))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	c.ApplyDefaults()
	return &c, nil
}

// Validate checks semantic rules. Field-level errors reference artifact names.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("version: expected 1, got %d (this release of shipsmoke reads schema version 1)", c.Version)
	}
	if len(c.Artifacts) == 0 {
		return fmt.Errorf("no artifacts defined: add at least one entry under \"artifacts:\"")
	}
	seen := map[string]bool{}
	for i := range c.Artifacts {
		a := &c.Artifacts[i]
		label := a.Name
		if label == "" {
			label = fmt.Sprintf("artifacts[%d]", i)
		}
		if a.Path == "" {
			return fmt.Errorf("%s: \"path\" is required", label)
		}
		if a.Binary == "" {
			return fmt.Errorf("%s: \"binary\" is required (relative path of the entry program, e.g. \"mytool\" or \"bin/mytool.exe\")", label)
		}
		if seen[a.Name] {
			return fmt.Errorf("duplicate artifact name %q: artifact names must be unique", a.Name)
		}
		seen[a.Name] = true
		switch a.Format {
		case "", "auto", "zip", "tar.gz", "binary":
		default:
			return fmt.Errorf("%s: unknown format %q (supported: auto, zip, tar.gz, binary)", label, a.Format)
		}
		switch a.Env {
		case "", "minimal", "inherit":
		default:
			return fmt.Errorf("%s: unknown env mode %q (supported: minimal, inherit)", label, a.Env)
		}
		for k := range a.EnvExtra {
			if !envKeyRe.MatchString(k) {
				return fmt.Errorf("%s: env_extra key %q is not a valid environment variable name", label, k)
			}
		}
		for _, r := range a.RunsOn {
			if r != "any" && !platformSpecRe.MatchString(r) {
				return fmt.Errorf("%s: invalid runs_on entry %q (use \"os/arch\" like linux/amd64, windows/amd64, darwin/arm64, or \"any\")", label, r)
			}
		}
		if len(a.Checks) == 0 {
			return fmt.Errorf("%s: no checks defined: add at least one entry under \"checks:\"", label)
		}
		for j := range a.Checks {
			ch := &a.Checks[j]
			if len(ch.Args) == 0 {
				return fmt.Errorf("%s: checks[%d]: \"args\" must list at least one argument (e.g. [\"--version\"])", label, j)
			}
			e := ch.Expect
			if e.ExitCode < 0 {
				return fmt.Errorf("%s: checks[%d]: expect.exit_code cannot be negative", label, j)
			}
			for _, field := range []struct {
				name  string
				value string
			}{
				{"stdout_regex", e.StdoutRegex},
				{"stderr_regex", e.StderrRegex},
			} {
				if field.value == "" {
					continue
				}
				if _, err := regexpCompile(field.value); err != nil {
					return fmt.Errorf("%s: checks[%d]: expect.%s is not a valid regular expression: %v", label, j, field.name, err)
				}
			}
		}
	}
	return nil
}
