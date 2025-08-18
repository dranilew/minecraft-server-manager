package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/dranilew/minecraft-server-manager/src/lib/run"
	"gopkg.in/yaml.v3"
)

var (
	// extraScriptsDir is the directory containing extra server-specific scripts.
	extraScriptsDir = "scripts"
	// configurationFile is the expected configuration file name.
	configurationFile = "scripts.yaml"
	// currentConfigurations is the set of configurations that the server is currently managing.
	currentConfigurations = make(map[string]*configuration)
)

// configuration is the configuration file.
type configuration struct {
	// Scripts is the list of scripts to run.
	Scripts []extraScript `yaml:"scripts"`
}

// extraScript contains the configuration for a single script.
// All scripts should be executable.
type extraScript struct {
	// Interval is interval on which the script should be executed.
	Interval time.Duration `yaml:"interval"`
	// Name is the name of the script. This should be the name of the file to be executed.
	Name string `yaml:"name"`
	// LastRun stores the last time the script was run.
	LastRun time.Time `yaml:"last-run,omitempty"`
}

// getScriptNames gets the list of all scripts from the configuration.
// The list of strings is the ordered list of script names, and the map
// maps the script names to their extraScript structs.
func getScriptNames(conf *configuration) ([]string, map[string]extraScript) {
	keys := make([]string, len(conf.Scripts))
	scriptNames := make(map[string]extraScript)

	for i, script := range conf.Scripts {
		keys[i] = script.Name
		scriptNames[script.Name] = script
	}
	return keys, scriptNames
}

// readYaml reads the configuration from the server directory and unmarshals it into
// the configuration struct. It then updates the currently stored configuration in
// memory with new scripts gotten from the newly read configuration, and deletes any
// existing configurations that no longer exist in the newly read configuration.
//
// This is done so that the server can keep track of the last runtime of a script
// without having to constantly write and read a file.
func readYaml(server string) error {
	// Read the configuration file.
	confFile := filepath.Join(common.ServerDirectory(server), configurationFile)
	contentBytes, err := os.ReadFile(confFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to read configuration file: %v", err)
		}
		logger.Debugf("Extra scripts configuration not found for %q, skipping", server)
		return nil
	}

	// Unmarshal the contents.
	var conf configuration
	if err := yaml.Unmarshal(contentBytes, &conf); err != nil {
		return fmt.Errorf("failed to unmarshal %q: %v", confFile, err)
	}

	// If the configuration doesn't exist, initialize and return.
	currConf, ok := currentConfigurations[server]
	if !ok {
		currentConfigurations[server] = &conf
		return nil
	}

	currConfKeys, _ := getScriptNames(currConf)
	gotConfKeys, gotConfScripts := getScriptNames(&conf)

	// Add new scripts.
	for scriptName, script := range gotConfScripts {
		if !slices.Contains(currConfKeys, scriptName) {
			currConf.Scripts = append(conf.Scripts, script)
		}
	}

	// Remove old scripts.
	var rmScripts []string
	for _, scriptName := range currConfKeys {
		if !slices.Contains(gotConfKeys, scriptName) {
			rmScripts = append(rmScripts, scriptName)
		}
	}
	currConf.Scripts = slices.DeleteFunc(currConf.Scripts, func(script extraScript) bool {
		return slices.Contains(rmScripts, script.Name)
	})

	// Update the current configuration.
	currentConfigurations[server] = currConf
	return nil
}

// ExtraScripts runs all the scripts in the `extra-scripts` directory.
// These can be configured to run on a specific timer. Typically these
// should only be run for servers that are actually running. These are
// run from the base server directory of the modpack.
func ExtraScripts(ctx context.Context, server string) error {
	// Make sure we have the most update-to-date configuration.
	if err := readYaml(server); err != nil {
		return fmt.Errorf("failed to read yaml file: %v", err)
	}

	// Run all the scripts configured by the configuration file.
	var errs []error
	for _, script := range currentConfigurations[server].Scripts {
		serverDir := common.ServerDirectory(server)
		scriptPath := filepath.Join(serverDir, extraScriptsDir, script.Name)

		// Only run when its next scheduled time to run has passed.
		if time.Since(script.LastRun) >= script.Interval {
			logger.Debugf("Running scripts %s for server %s", script.Name, server)
			script.LastRun = time.Now()
			opts := run.Options{
				Name:       scriptPath,
				OutputType: run.OutputNone,
				ExecMode:   run.ExecModeAsync,
				Dir:        serverDir,
			}
			if _, err := run.WithContext(ctx, opts); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
