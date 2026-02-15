package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Types — Flipt feature flag file schema
// ---------------------------------------------------------------------------

type FeaturesFile struct {
	Namespace Namespace `yaml:"namespace"`
	Flags     []Flag    `yaml:"flags"`
	Segments  []Segment `yaml:"segments"`
}

type Namespace struct {
	Key         string `yaml:"key"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Flag struct {
	Key         string    `yaml:"key"`
	Name        string    `yaml:"name"`
	Type        string    `yaml:"type"`
	Description string    `yaml:"description"`
	Enabled     bool      `yaml:"enabled"`
	Rollouts    []Rollout `yaml:"rollouts"`
	Variants    []Variant `yaml:"variants"`
	Rules       []Rule    `yaml:"rules"`
	Metadata    any       `yaml:"metadata"`
}

type Rollout struct {
	Segment   *SegmentRef `yaml:"segment"`
	Threshold *Threshold  `yaml:"threshold"`
}

type SegmentRef struct {
	Key   string   `yaml:"key"`
	Keys  []string `yaml:"keys"`
	Value any      `yaml:"value"`
}

type Threshold struct {
	Percentage float64 `yaml:"percentage"`
	Value      any     `yaml:"value"`
}

type Variant struct {
	Key         string `yaml:"key"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Default     bool   `yaml:"default"`
	Attachment  any    `yaml:"attachment"`
}

type Rule struct {
	Segment       *SegmentRef    `yaml:"segment"`
	Distributions []Distribution `yaml:"distributions"`
}

type Distribution struct {
	Variant string  `yaml:"variant"`
	Rollout float64 `yaml:"rollout"`
}

type Segment struct {
	Key         string       `yaml:"key"`
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Constraints []Constraint `yaml:"constraints"`
	MatchType   string       `yaml:"match_type"`
}

type Constraint struct {
	Type        string `yaml:"type"`
	Property    string `yaml:"property"`
	Operator    string `yaml:"operator"`
	Value       string `yaml:"value"`
	Description string `yaml:"description"`
}

type AccessFile struct {
	Writers []string `yaml:"writers"`
}

// ---------------------------------------------------------------------------
// Issue — a single lint finding
// ---------------------------------------------------------------------------

const (
	levelError   = 0
	levelWarning = 1
)

type issue struct {
	message string
	level   int
}

func errorf(format string, args ...any) issue {
	return issue{message: fmt.Sprintf(format, args...), level: levelError}
}

func warnf(format string, args ...any) issue {
	return issue{message: fmt.Sprintf(format, args...), level: levelWarning}
}

// ---------------------------------------------------------------------------
// lintFeaturesFile — structural validation of a features.yml
// ---------------------------------------------------------------------------

func lintFeaturesFile(path string, data []byte) []issue {
	var file FeaturesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return []issue{errorf("invalid YAML: %v", err)}
	}

	var issues []issue

	// Required namespace fields
	if file.Namespace.Key == "" {
		issues = append(issues, errorf("missing required field: namespace.key"))
	}
	if file.Namespace.Name == "" {
		issues = append(issues, errorf("missing required field: namespace.name"))
	}

	// Build segment lookup
	segmentKeys := make(map[string]bool)
	segmentDuplicates := make(map[string]bool)
	for _, seg := range file.Segments {
		if segmentKeys[seg.Key] {
			issues = append(issues, errorf("segment %q: duplicate key", seg.Key))
			segmentDuplicates[seg.Key] = true
		}
		segmentKeys[seg.Key] = true
	}

	// Build variant lookup per flag, check flags
	flagKeys := make(map[string]bool)
	referencedSegments := make(map[string]bool)

	for _, f := range file.Flags {
		// Required flag fields
		if f.Key == "" {
			issues = append(issues, errorf("flag with empty key"))
			continue
		}
		if f.Name == "" {
			issues = append(issues, errorf("flag %q: missing required field: name", f.Key))
		}
		if f.Type == "" {
			issues = append(issues, errorf("flag %q: missing required field: type", f.Key))
		}

		// Duplicate flag keys
		if flagKeys[f.Key] {
			issues = append(issues, errorf("flag %q: duplicate key", f.Key))
		}
		flagKeys[f.Key] = true

		// Invalid flag type
		if f.Type != "" && f.Type != "BOOLEAN_FLAG_TYPE" && f.Type != "VARIANT_FLAG_TYPE" {
			issues = append(issues, errorf("flag %q: invalid type %q (must be BOOLEAN_FLAG_TYPE or VARIANT_FLAG_TYPE)", f.Key, f.Type))
		}

		// Type/field mismatch
		if f.Type == "VARIANT_FLAG_TYPE" && len(f.Rollouts) > 0 {
			issues = append(issues, errorf("flag %q: variant flag cannot have rollouts", f.Key))
		}
		if f.Type == "BOOLEAN_FLAG_TYPE" {
			if len(f.Variants) > 0 {
				issues = append(issues, errorf("flag %q: boolean flag cannot have variants", f.Key))
			}
			if len(f.Rules) > 0 {
				issues = append(issues, errorf("flag %q: boolean flag cannot have rules", f.Key))
			}
		}

		// Collect segment refs and check they exist
		segRefs := collectSegmentRefs(f)
		for _, ref := range segRefs {
			referencedSegments[ref] = true
			if !segmentKeys[ref] {
				issues = append(issues, errorf("flag %q: references segment %q which is not defined", f.Key, ref))
			}
		}

		// Check variant refs in rule distributions
		if f.Type == "VARIANT_FLAG_TYPE" {
			variantKeys := make(map[string]bool)
			for _, v := range f.Variants {
				variantKeys[v.Key] = true
			}
			for _, rule := range f.Rules {
				for _, dist := range rule.Distributions {
					if !variantKeys[dist.Variant] {
						issues = append(issues, errorf("flag %q: distribution references variant %q which is not defined", f.Key, dist.Variant))
					}
				}
			}
		}
	}

	// Unused segments (warning)
	for _, seg := range file.Segments {
		if !referencedSegments[seg.Key] && !segmentDuplicates[seg.Key] {
			issues = append(issues, warnf("segment %q is defined but not referenced by any flag", seg.Key))
		}
	}

	return issues
}

// collectSegmentRefs extracts all segment keys referenced by a flag's rollouts and rules.
// Handles both single `key` and multi `keys` syntax.
func collectSegmentRefs(f Flag) []string {
	var refs []string
	for _, r := range f.Rollouts {
		if r.Segment != nil {
			if r.Segment.Key != "" {
				refs = append(refs, r.Segment.Key)
			}
			refs = append(refs, r.Segment.Keys...)
		}
	}
	for _, rule := range f.Rules {
		if rule.Segment != nil {
			if rule.Segment.Key != "" {
				refs = append(refs, rule.Segment.Key)
			}
			refs = append(refs, rule.Segment.Keys...)
		}
	}
	return refs
}

// ---------------------------------------------------------------------------
// lintAccessFile — validates an access.yml
// ---------------------------------------------------------------------------

func lintAccessFile(path string, data []byte) []issue {
	var file AccessFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return []issue{errorf("invalid YAML: %v", err)}
	}

	if len(file.Writers) == 0 {
		return []issue{errorf("missing required field: writers")}
	}

	return nil
}

// ---------------------------------------------------------------------------
// checkFormatting — round-trip formatting check
// ---------------------------------------------------------------------------

func checkFormatting(path string, data []byte) []issue {
	canonical, err := roundTrip(data)
	if err != nil {
		return []issue{errorf("invalid YAML: %v", err)}
	}

	origLines := strings.Split(string(bytes.TrimRight(data, "\n")), "\n")
	canonLines := strings.Split(string(bytes.TrimRight(canonical, "\n")), "\n")

	var issues []issue

	if len(origLines) != len(canonLines) {
		// Find extra blank lines by walking the original and flagging lines
		// that don't appear in the canonical output.
		ci := 0
		for oi := 0; oi < len(origLines); oi++ {
			if ci < len(canonLines) && origLines[oi] == canonLines[ci] {
				ci++
				continue
			}
			if strings.TrimSpace(origLines[oi]) == "" {
				issues = append(issues, errorf("formatting: line %d is an extra blank line", oi+1))
			} else {
				issues = append(issues, errorf("formatting: line %d differs from canonical form", oi+1))
			}
		}
		if len(issues) == 0 {
			issues = append(issues, errorf("formatting: expected %d lines, got %d", len(canonLines), len(origLines)))
		}
		return issues
	}

	for i := 0; i < len(origLines); i++ {
		if origLines[i] != canonLines[i] {
			issues = append(issues, errorf("formatting: line %d differs from canonical form", i+1))
		}
	}

	return issues
}

// ---------------------------------------------------------------------------
// roundTrip — canonical YAML re-serialization (4-space indent)
// ---------------------------------------------------------------------------

func roundTrip(data []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(4)

	if err := encoder.Encode(&node); err != nil {
		return nil, err
	}

	encoder.Close()
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// fixFile — reformat a file to canonical YAML (formatting only)
// ---------------------------------------------------------------------------

func fixFile(path string) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	canonical, err := roundTrip(original)
	if err != nil {
		return err
	}

	if bytes.Equal(bytes.TrimRight(original, "\n"), bytes.TrimRight(canonical, "\n")) {
		return nil
	}

	return os.WriteFile(path, canonical, 0644)
}

// ---------------------------------------------------------------------------
// main — flag parsing, file discovery, orchestration, reporting
// ---------------------------------------------------------------------------

func main() {
	fix := flag.Bool("fix", false, "reformat files in place instead of just checking")
	flag.Parse()

	cfg := zap.NewProductionConfig()
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05Z")
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	logger, _ := cfg.Build()
	defer logger.Sync()

	args := flag.Args()
	if len(args) == 0 {
		logger.Fatal("invalid arguments", zap.String("usage", "lint-flags [--fix] <flags-dir>"))
	}

	flagsDir := args[0]

	// Discover files
	patterns := []string{
		filepath.Join(flagsDir, "*", "*", "features.yml"),
		filepath.Join(flagsDir, "*", "*", "features.yaml"),
		filepath.Join(flagsDir, "*", "*", "access.yml"),
	}

	var files []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		files = append(files, matches...)
	}
	sort.Strings(files)

	if len(files) == 0 {
		logger.Warn("no flag files found", zap.String("path", flagsDir))
		return
	}

	// Fix mode — formatting only
	if *fix {
		for _, path := range files {
			rel, _ := filepath.Rel(flagsDir, path)
			if rel == "" {
				rel = path
			}

			if err := fixFile(path); err != nil {
				logger.Error("failed to fix file", zap.String("path", rel), zap.Error(err))
			} else {
				logger.Info("formatted", zap.String("path", rel))
			}
		}
		return
	}

	// Lint mode — full validation
	totalErrors := 0
	totalWarnings := 0
	filesWithIssues := make(map[string][]issue)

	for _, path := range files {
		rel, _ := filepath.Rel(flagsDir, path)
		if rel == "" {
			rel = path
		}

		data, err := os.ReadFile(path)
		if err != nil {
			filesWithIssues[rel] = append(filesWithIssues[rel], errorf("cannot read file: %v", err))
			totalErrors++
			continue
		}

		var fileIssues []issue

		basename := filepath.Base(path)
		if basename == "access.yml" {
			fileIssues = append(fileIssues, lintAccessFile(path, data)...)
			fileIssues = append(fileIssues, checkFormatting(path, data)...)
		} else {
			fileIssues = append(fileIssues, lintFeaturesFile(path, data)...)
			fileIssues = append(fileIssues, checkFormatting(path, data)...)
		}

		if len(fileIssues) > 0 {
			filesWithIssues[rel] = fileIssues
			for _, iss := range fileIssues {
				if iss.level == levelError {
					totalErrors++
				} else {
					totalWarnings++
				}
			}
		}
	}

	// Report — grouped by file, errors first, then warnings
	if len(filesWithIssues) > 0 {
		// Sort file paths for deterministic output
		var sortedFiles []string
		for f := range filesWithIssues {
			sortedFiles = append(sortedFiles, f)
		}
		sort.Strings(sortedFiles)

		fmt.Fprintln(os.Stderr)
		for _, rel := range sortedFiles {
			issues := filesWithIssues[rel]
			fmt.Fprintf(os.Stderr, "%s\n", rel)

			// Print errors first, then warnings
			for _, iss := range issues {
				if iss.level == levelError {
					fmt.Fprintf(os.Stderr, "  ERROR  %s\n", iss.message)
				}
			}
			for _, iss := range issues {
				if iss.level == levelWarning {
					fmt.Fprintf(os.Stderr, "  WARN   %s\n", iss.message)
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	}

	// Summary
	if totalErrors > 0 {
		logger.Error(fmt.Sprintf("lint complete: %d files checked, %d errors, %d warnings", len(files), totalErrors, totalWarnings))
		os.Exit(1)
	}

	if totalWarnings > 0 {
		logger.Warn(fmt.Sprintf("lint complete: %d files checked, 0 errors, %d warnings", len(files), totalWarnings))
		return
	}

	logger.Info(fmt.Sprintf("lint passed: %d files checked", len(files)))
}
