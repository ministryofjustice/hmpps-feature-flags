package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type featuresFile struct {
	Namespace struct {
		Key string `yaml:"key"`
	} `yaml:"namespace"`
}

type accessFile struct {
	Writers []string `yaml:"writers"`
}

type aclData struct {
	NamespaceTeamAccess map[string][]string `json:"namespace_team_access"`
}

// readNamespaceKey looks for a features.yml/yaml in the given directory and
// extracts the Flipt namespace key. Falls back to "" if no file is found,
// since the directory name may differ from the actual namespace key
// (e.g. directory "probation-in-court" → namespace "ProbationInCourt").
func readNamespaceKey(nsDir string) string {
	for _, name := range []string{"features.yml", "features.yaml"} {
		path := filepath.Join(nsDir, name)
		data, err := os.ReadFile(path)

		if err != nil {
			continue
		}

		var f featuresFile
		if err := yaml.Unmarshal(data, &f); err == nil && f.Namespace.Key != "" {
			return f.Namespace.Key
		}
	}
	return ""
}

// generate reads all access.yml files under flags/<env>/<namespace>/,
// builds a JSON map of namespace → writer teams, and writes it atomically
// to outputPath. This JSON is consumed by Flipt's OPA authorization policy
// to determine which GitHub teams can write to which namespaces.
func generate(logger *zap.Logger, flagsDir string, outputPath string, msg string) error {
	matches, _ := filepath.Glob(filepath.Join(flagsDir, "*", "*", "access.yml"))
	sort.Strings(matches)

	result := aclData{NamespaceTeamAccess: make(map[string][]string)}

	for _, accessPath := range matches {
		nsDir := filepath.Dir(accessPath)

		namespace := readNamespaceKey(nsDir)
		if namespace == "" {
			namespace = filepath.Base(nsDir)
		}

		if _, exists := result.NamespaceTeamAccess[namespace]; exists {
			continue
		}

		data, err := os.ReadFile(accessPath)
		if err != nil {
			logger.Warn("failed to read access file", zap.String("path", accessPath), zap.Error(err))
			continue
		}

		var af accessFile
		if err := yaml.Unmarshal(data, &af); err != nil || len(af.Writers) == 0 {
			logger.Warn("skipping access file with no writers", zap.String("path", accessPath))
			continue
		}

		result.NamespaceTeamAccess[namespace] = af.Writers
	}

	out, _ := json.MarshalIndent(result, "", "  ")

	tmpPath := outputPath + ".tmp"

	if err := os.WriteFile(tmpPath, append(out, '\n'), 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		return err
	}

	logger.Info(msg, zap.String("path", outputPath))
	return nil
}

func main() {
	watch := flag.Bool("watch", false, "poll for file changes and regenerate ACL data")
	interval := flag.Duration("interval", 15*time.Second, "poll interval when using --watch")
	flag.Parse()

	cfg := zap.NewProductionConfig()
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05Z")
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.DisableCaller = true
	logger, _ := cfg.Build()
	defer logger.Sync()

	args := flag.Args()
	if len(args) != 2 {
		logger.Fatal("invalid arguments", zap.String("usage", "generate-acl-data [--watch] [--interval 15s] <flags-dir> <output-path>"))
	}

	flagsDir := args[0]
	outputPath := args[1]

	if err := generate(logger, flagsDir, outputPath, "generated ACL data"); err != nil {
		logger.Fatal("failed to generate ACL data", zap.Error(err))
	}

	if !*watch {
		return
	}

	logger.Info("polling for changes", zap.String("path", flagsDir), zap.Duration("interval", *interval))

	var lastOutput []byte

	for {
		time.Sleep(*interval)

		current, err := os.ReadFile(outputPath)
		if err != nil {
			logger.Warn("failed to read current ACL data", zap.Error(err))
		}

		// Regenerate into a temporary buffer to compare
		matches, _ := filepath.Glob(filepath.Join(flagsDir, "*", "*", "access.yml"))
		if len(matches) == 0 {
			continue
		}

		if err := generate(logger, flagsDir, outputPath, "refreshed ACL data"); err != nil {
			logger.Error("failed to regenerate ACL data", zap.Error(err))
			continue
		}

		newOutput, _ := os.ReadFile(outputPath)

		if lastOutput == nil {
			lastOutput = current
		}

		if string(newOutput) != string(lastOutput) {
			logger.Info("ACL data changed, written to disk", zap.String("path", outputPath))
		}

		lastOutput = newOutput
	}
}
