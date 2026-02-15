package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ANSI colour codes
const (
	bold   = "\033[1m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

var (
	scanner    = bufio.NewScanner(os.Stdin)
	kebabRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	envs       = []string{"dev", "preprod", "prod"}
)

func info(msg string)  { fmt.Printf("%s%s%s\n", cyan, msg, reset) }
func ok(msg string)    { fmt.Printf("%s%s%s\n", green, msg, reset) }
func warn(msg string)  { fmt.Printf("%s%s%s\n", yellow, msg, reset) }
func fail(msg string)  { fmt.Fprintf(os.Stderr, "%s%s%s\n", red, msg, reset) }

func prompt(label string, defaultValue string) string {
	for {
		if defaultValue != "" {
			fmt.Printf("%s%s%s [%s%s%s]: ", bold, label, reset, cyan, defaultValue, reset)
		} else {
			fmt.Printf("%s%s%s: ", bold, label, reset)
		}

		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			input = defaultValue
		}

		if input != "" {
			return input
		}

		fail("This field is required.")
	}
}

func optionalPrompt(label string) string {
	fmt.Printf("%s%s%s: ", bold, label, reset)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptList(label string) []string {
	var items []string

	for {
		prefix := fmt.Sprintf("  %d. ", len(items)+1)
		if len(items) == 0 {
			fmt.Printf("%s%s%s\n", bold, label, reset)
		}

		fmt.Printf("%s", prefix)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			if len(items) == 0 {
				fail("At least one entry is required.")
				continue
			}
			return items
		}

		items = append(items, input)
		info("  (press enter to finish)")
	}
}

// updateCodeowners adds a CODEOWNERS entry for the new namespace, keeping
// the namespace entries sorted alphabetically by path.
func updateCodeowners(flagsDir string, nsKey string, ghTeams []string) error {
	codeownersPath := filepath.Join(flagsDir, "..", ".github", "CODEOWNERS")

	data, err := os.ReadFile(codeownersPath)
	if err != nil {
		return fmt.Errorf("could not read CODEOWNERS: %w", err)
	}

	// Build the new entry
	var owners []string
	for _, team := range ghTeams {
		owners = append(owners, "@ministryofjustice/"+team)
	}

	newPath := "flags/*/" + nsKey + "/"
	newEntry := newPath + " " + strings.Join(owners, " ")

	// Split into header (everything before namespace entries) and namespace entries
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	var header []string
	var entries []string
	entryPrefix := "flags/*/"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, entryPrefix) {
			entries = append(entries, trimmed)
		} else {
			header = append(header, line)
		}
	}

	entries = append(entries, newEntry)
	sort.Strings(entries)

	// Pad entries so the @ owners are column-aligned
	maxPathLen := 0
	for _, entry := range entries {
		pathEnd := strings.Index(entry, "@")
		if pathEnd > maxPathLen {
			maxPathLen = pathEnd
		}
	}

	var aligned []string
	for _, entry := range entries {
		pathEnd := strings.Index(entry, "@")
		path := strings.TrimRight(entry[:pathEnd], " ")
		owners := entry[pathEnd:]
		padding := strings.Repeat(" ", maxPathLen-len(path))
		aligned = append(aligned, path+padding+owners)
	}

	output := strings.Join(header, "\n") + "\n" + strings.Join(aligned, "\n") + "\n"

	return os.WriteFile(codeownersPath, []byte(output), 0644)
}

func confirm(label string) bool {
	fmt.Printf("%s%s%s ", bold, label, reset)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	if input == "" {
		input = "Y"
	}

	return strings.ToLower(input[:1]) == "y"
}

func main() {
	if len(os.Args) != 2 {
		fail("Usage: new-namespace <flags-dir>")
		os.Exit(1)
	}

	flagsDir := os.Args[1]

	fmt.Println()
	info("============================================")
	info("  Create a new Flipt namespace")
	info("============================================")
	fmt.Println()
	info("This will scaffold a new namespace across all")
	info("environments (dev, preprod, prod) with the")
	info("required features.yml and access.yml files.")
	fmt.Println()

	// --- Gather inputs ---

	nsKey := strings.ToLower(strings.ReplaceAll(prompt("Namespace key (kebab-case, e.g. my-service)", ""), " ", "-"))

	if !kebabRegex.MatchString(nsKey) {
		fail("Namespace key must be kebab-case (lowercase letters, numbers, hyphens).")
		os.Exit(1)
	}

	for _, env := range envs {
		if _, err := os.Stat(filepath.Join(flagsDir, env, nsKey)); err == nil {
			fail(fmt.Sprintf("Namespace '%s' already exists in %s!", nsKey, env))
			os.Exit(1)
		}
	}

	nsName := prompt("Display name", nsKey)
	nsDesc := optionalPrompt("Description (optional)")

	fmt.Println()
	ghTeams := promptList("GitHub team slugs for write access:")

	// --- Summary ---

	fmt.Println()
	info("============================================")
	info("  Summary")
	info("============================================")
	fmt.Println()
	fmt.Printf("  %sNamespace:%s    %s\n", bold, reset, nsKey)
	if nsName != "" {
		fmt.Printf("  %sName:%s         %s\n", bold, reset, nsName)
	}
	if nsDesc != "" {
		fmt.Printf("  %sDescription:%s  %s\n", bold, reset, nsDesc)
	}
	fmt.Printf("  %sTeams:%s        %s\n", bold, reset, strings.Join(ghTeams, ", "))
	fmt.Printf("  %sEnvironments:%s %s\n", bold, reset, strings.Join(envs, ", "))
	fmt.Println()

	if !confirm("Create this namespace? [Y/n]:") {
		warn("Aborted.")
		return
	}

	// --- Create files ---

	fmt.Println()
	for _, env := range envs {
		dir := filepath.Join(flagsDir, env, nsKey)

		if err := os.MkdirAll(dir, 0755); err != nil {
			fail(fmt.Sprintf("Failed to create directory %s: %v", dir, err))
			os.Exit(1)
		}

		features := "namespace:\n    key: " + nsKey + "\n"
		if nsName != "" {
			features += "    name: " + nsName + "\n"
		}
		if nsDesc != "" {
			features += "    description: " + nsDesc + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "features.yml"), []byte(features), 0644); err != nil {
			fail(fmt.Sprintf("Failed to write features.yml in %s: %v", env, err))
			os.Exit(1)
		}

		access := "writers:\n"
		for _, team := range ghTeams {
			access += "    - " + team + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "access.yml"), []byte(access), 0644); err != nil {
			fail(fmt.Sprintf("Failed to write access.yml in %s: %v", env, err))
			os.Exit(1)
		}

		ok(fmt.Sprintf("  Created %s/%s/", env, nsKey))
	}

	// --- Update CODEOWNERS ---

	if err := updateCodeowners(flagsDir, nsKey, ghTeams); err != nil {
		fail(fmt.Sprintf("Failed to update CODEOWNERS: %v", err))
		os.Exit(1)
	}

	ok("  Updated .github/CODEOWNERS")

	fmt.Println()
	ok(fmt.Sprintf("Namespace '%s' created in all environments.", nsKey))
	fmt.Println()
	info("Next steps:")
	info("  1. Run 'make flags-lint' to validate")
	info("  2. Add your flags to the features.yml files")
	info("  3. Raise a PR to main")
	fmt.Println()
}
