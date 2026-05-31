package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	rcconfig "rootcause/internal/config"
)

const defaultConfigFileName = "config.yaml"

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
	// 0o700 because config.yaml inside is 0o600 and may carry paths to
	// service-account keys / credentials. Keep the parent private to the
	// user, matching standard ~/.ssh, ~/.aws layout.
	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	cfg := rcconfig.DefaultConfig()
	cfg.ReadOnly = false
	cfg.DisableDestructive = false
	cfg.Skills.CustomDirs = []string{"~/.rootcause/skills"}
	cfg.Prompts.Dir = "~/.rootcause/prompts"
	data, err := encodeConfig(cfg)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	err = os.MkdirAll(filepath.Join(filepath.Dir(path), "skills"), 0o700)
	if err != nil {
		return "", fmt.Errorf("create custom skills directory: %w", err)
	}
	err = os.MkdirAll(filepath.Join(filepath.Dir(path), "prompts"), 0o700)
	if err != nil {
		return "", fmt.Errorf("create custom prompts directory: %w", err)
	}
	return path, nil
}

func encodeConfig(cfg rcconfig.Config) ([]byte, error) {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	header := []byte("# RootCause configuration. Edit and reload (or restart the MCP server).\n# See README \"Config and Flags\" + \"GCP/AWS Credentials\" sections for details.\n\n")
	return append(header, out...), nil
}
