package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	rcconfig "rootcause/internal/config"
)

const defaultConfigFileName = "config.toml"

func newInitConfigCmd(stderr io.Writer) *cobra.Command {
	var overwrite bool
	var path string

	cmd := &cobra.Command{
		Use:   "init-config",
		Short: "Initialize a RootCause config in your home directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			configPath := path
			if configPath == "" {
				var err error
				configPath, err = defaultHomeConfigPath()
				if err != nil {
					return err
				}
			}
			writtenPath, err := initHomeConfig(configPath, overwrite)
			if err != nil {
				return err
			}
			if stderr == nil {
				stderr = os.Stdout
			}
			_, _ = fmt.Fprintf(stderr, "Initialized RootCause config: %s\n", writtenPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite an existing home config")
	cmd.Flags().StringVar(&path, "path", "", "write config to this path instead of the home config path")
	_ = cmd.Flags().MarkHidden("path")
	return cmd
}

func defaultHomeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return homeConfigPath(home), nil
}

func homeConfigPath(home string) string {
	return filepath.Join(home, ".rootcause", defaultConfigFileName)
}

func initHomeConfig(path string, overwrite bool) (string, error) {
	if !overwrite {
		_, err := os.Stat(path)
		if err == nil {
			return "", fmt.Errorf("config already exists at %s; pass --overwrite to replace it", path)
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("check existing config: %w", err)
		}
	}
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	cfg := rcconfig.DefaultConfig()
	cfg.ReadOnly = false
	cfg.DisableDestructive = false
	cfg.Skills.CustomDirs = []string{"~/.rootcause/skills"}
	data, err := encodeConfig(cfg)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	err = os.MkdirAll(filepath.Join(filepath.Dir(path), "skills"), 0o755)
	if err != nil {
		return "", fmt.Errorf("create custom skills directory: %w", err)
	}
	return path, nil
}

func encodeConfig(cfg rcconfig.Config) ([]byte, error) {
	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return buf.Bytes(), nil
}
