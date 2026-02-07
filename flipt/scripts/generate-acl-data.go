package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
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

// watchDirs recursively registers all directories under flagsDir with
// the fsnotify watcher, so we detect changes at any depth (e.g. new
// namespaces or updated access.yml files after a git pull).
func watchDirs(watcher *fsnotify.Watcher, flagsDir string) {
	watcher.Add(flagsDir)

	filepath.Walk(flagsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			watcher.Add(path)
		}

		return nil
	})
}

func main() {
	watch := flag.Bool("watch", false, "watch for file changes and regenerate ACL data")
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
		logger.Fatal("invalid arguments", zap.String("usage", "generate-acl-data [--watch] <flags-dir> <output-path>"))
	}

	flagsDir := args[0]
	outputPath := args[1]

	if err := generate(logger, flagsDir, outputPath, "generated ACL data"); err != nil {
		logger.Fatal("failed to generate ACL data", zap.Error(err))
	}

	if !*watch {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatal("failed to create file watcher", zap.Error(err))
	}
	defer watcher.Close()

	watchDirs(watcher, flagsDir)
	logger.Info("watching for changes", zap.String("path", flagsDir))

	// Debounce: git pulls trigger many file events at once
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			// Watch newly created directories
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watchDirs(watcher, event.Name)
				}
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			debounceTimer = time.AfterFunc(2*time.Second, func() {
				if err := generate(logger, flagsDir, outputPath, "refreshed ACL data"); err != nil {
					logger.Error("failed to regenerate ACL data", zap.Error(err))
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			logger.Error("file watcher error", zap.Error(err))
		}
	}
}
