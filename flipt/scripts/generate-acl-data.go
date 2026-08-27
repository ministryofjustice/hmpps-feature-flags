package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
	AuthzConfig         authzConfig                    `json:"authz_config,omitempty"`
	NamespaceTeamAccess map[string]map[string][]string `json:"namespace_team_access"`
}

type authzConfig struct {
	DefaultEnvironment string `json:"default_environment,omitempty"`
}

// flagSource abstracts where flag configuration is read from. Flipt's git
// storage fetches new commits without updating the checked-out files baked
// into the image, so a running instance must read from a git ref to see
// changes merged after the image was built. The filesystem source covers
// image builds and local runs, where there is no fetched ref to read.
type flagSource interface {
	accessPaths() ([]string, error)
	read(path string) ([]byte, error)
}

type filesystemSource struct {
	flagsDir string
}

func (s filesystemSource) accessPaths() ([]string, error) {
	return filepath.Glob(filepath.Join(s.flagsDir, "*", "*", "access.yml"))
}

func (s filesystemSource) read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type gitSource struct {
	gitDir string
	ref    string
}

func (s gitSource) accessPaths() ([]string, error) {
	// #nosec G204 -- gitDir and ref come from trusted process arguments
	out, err := exec.Command("git", "-C", s.gitDir, "ls-tree", "-r", "--name-only", s.ref, "--", "flags").Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, "/")
		if len(parts) == 4 && parts[3] == "access.yml" {
			paths = append(paths, line)
		}
	}

	return paths, nil
}

func (s gitSource) read(path string) ([]byte, error) {
	// #nosec G204 -- gitDir and ref come from trusted process arguments
	return exec.Command("git", "-C", s.gitDir, "show", s.ref+":"+path).Output()
}

// resolveSource prefers reading from the git ref when one is configured and
// readable, and falls back to the filesystem otherwise (e.g. before Flipt's
// first fetch has created the ref).
func resolveSource(logger *zap.Logger, flagsDir string, gitRef string) flagSource {
	if gitRef == "" {
		return filesystemSource{flagsDir: flagsDir}
	}

	src := gitSource{gitDir: filepath.Dir(flagsDir), ref: gitRef}
	if _, err := src.accessPaths(); err != nil {
		logger.Warn("git ref unavailable, reading flags from filesystem",
			zap.String("ref", gitRef), zap.Error(err))
		return filesystemSource{flagsDir: flagsDir}
	}

	return src
}

func canonicalEnvironmentName(environment string) string {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production":
		return "prod"
	case "preprod", "pre-prod", "pre-production":
		return "preprod"
	default:
		return strings.ToLower(strings.TrimSpace(environment))
	}
}

// readNamespaceKey looks for a features.yml/yaml in the given directory and
// extracts the Flipt namespace key. Falls back to "" if no file is found,
// since the directory name may differ from the actual namespace key
// (e.g. directory "probation-in-court" → namespace "ProbationInCourt").
func readNamespaceKey(source flagSource, nsDir string) string {
	for _, name := range []string{"features.yml", "features.yaml"} {
		data, err := source.read(filepath.Join(nsDir, name))

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

// generate reads all access.yml files under flags/<env>/<namespace>/ from the
// given source, builds a JSON map of environment → namespace → writer teams,
// and writes it atomically to outputPath. This JSON is consumed by Flipt's OPA
// authorization policy to determine which GitHub teams can write to which
// namespaces in each environment.
func generate(logger *zap.Logger, source flagSource, outputPath string, msg string) error {
	matches, _ := source.accessPaths()
	sort.Strings(matches)

	result := aclData{
		AuthzConfig: authzConfig{
			DefaultEnvironment: canonicalEnvironmentName(os.Getenv("FLIPT_DEFAULT_ENVIRONMENT")),
		},
		NamespaceTeamAccess: make(map[string]map[string][]string),
	}

	for _, accessPath := range matches {
		nsDir := filepath.Dir(accessPath)
		envDir := filepath.Dir(nsDir)
		environment := canonicalEnvironmentName(filepath.Base(envDir))

		namespace := readNamespaceKey(source, nsDir)
		if namespace == "" {
			namespace = filepath.Base(nsDir)
		}

		if _, exists := result.NamespaceTeamAccess[environment]; !exists {
			result.NamespaceTeamAccess[environment] = make(map[string][]string)
		}

		data, err := source.read(accessPath)
		if err != nil {
			logger.Warn("failed to read access file", zap.String("path", accessPath), zap.Error(err))
			continue
		}

		var af accessFile
		if err := yaml.Unmarshal(data, &af); err != nil || len(af.Writers) == 0 {
			logger.Warn("skipping access file with no writers", zap.String("path", accessPath))
			continue
		}

		result.NamespaceTeamAccess[environment][namespace] = af.Writers
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
	gitRef := flag.String("git-ref", "", "read flags from this git ref in the repo containing <flags-dir>, falling back to the filesystem while the ref is unavailable")
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
		logger.Fatal("invalid arguments", zap.String("usage", "generate-acl-data [--watch] [--interval 15s] [--git-ref refs/remotes/origin/main] <flags-dir> <output-path>"))
	}

	flagsDir := args[0]
	outputPath := args[1]

	if err := generate(logger, resolveSource(logger, flagsDir, *gitRef), outputPath, "generated ACL data"); err != nil {
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

		// Re-resolve each cycle so the generator switches from the baked-in
		// filesystem copy to the git ref once Flipt's first fetch creates it.
		source := resolveSource(logger, flagsDir, *gitRef)

		matches, err := source.accessPaths()
		if err != nil || len(matches) == 0 {
			continue
		}

		if err := generate(logger, source, outputPath, "refreshed ACL data"); err != nil {
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
