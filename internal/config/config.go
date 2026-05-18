package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the full unicli configuration.
// Maps 1:1 to ~/.unicli/config.yaml
type Config struct {
	Alias    string         `mapstructure:"alias"`
	Download DownloadConfig `mapstructure:"download"`
	Engines  EnginesConfig  `mapstructure:"engines"`
}

type DownloadConfig struct {
	OutputDir      string `mapstructure:"output_dir"`
	DefaultQuality string `mapstructure:"default_quality"`
}

type EnginesConfig struct {
	BinDir        string `mapstructure:"bin_dir"`
	YtDlpPath     string `mapstructure:"ytdlp_path"`
	GalleryDlPath string `mapstructure:"gallerydl_path"`
}

// Dir returns the unicli config directory: ~/.unicli
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".unicli"), nil
}

// Load reads ~/.unicli/config.yaml into a Config struct.
// If the file doesn't exist, returns defaults without error.
func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)
	setDefaults(dir)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	return &cfg, nil
}

// Init creates ~/.unicli/config.yaml with default values.
// Safe to call if file already exists — does nothing.
func Init() error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	}

	setDefaults(dir)
	return viper.WriteConfigAs(cfgPath)
}

// SetAlias writes the alias field to ~/.unicli/config.yaml.
func SetAlias(name string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	cfgPath := filepath.Join(dir, "config.yaml")

	// Ensure config file exists before writing
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := Init(); err != nil {
			return err
		}
	}

	viper.Set("alias", name)
	return viper.WriteConfigAs(cfgPath)
}

// ClearAlias resets the alias field back to "unicli".
func ClearAlias() error {
	return SetAlias("unicli")
}

func setDefaults(dir string) {
	viper.SetDefault("alias", "unicli")
	viper.SetDefault("download.output_dir", ".")
	viper.SetDefault("download.default_quality", "best")
	viper.SetDefault("engines.bin_dir", filepath.Join(dir, "bin"))
	viper.SetDefault("engines.ytdlp_path", "")
	viper.SetDefault("engines.gallerydl_path", "")
}
