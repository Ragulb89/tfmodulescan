package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/olekukonko/tablewriter"
)

type ModuleInfo struct {
	ResourceName string
	ModuleName   string
	GitHubRepo   string
	Version      string
}

func main() {
	// Define flags
	dir := flag.String("dir", ".", "Directory to scan for Terraform modules (default: current directory)")
	excludeDirs := flag.String("exclude", "", "Comma-separated list of directories to exclude (e.g. .git,vendor,test)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of tfmodulescan:\n")
		flag.PrintDefaults()
		fmt.Println("\nExample:")
		fmt.Println("  tfmodulescan -dir ./infrastructure -exclude .git,vendor")
	}

	flag.Parse()

	// Parse exclusions into a set
	excluded := map[string]bool{}
	for _, d := range strings.Split(*excludeDirs, ",") {
		if trimmed := strings.TrimSpace(d); trimmed != "" {
			excluded[trimmed] = true
		}
	}

	var modules []ModuleInfo

	err := filepath.WalkDir(*dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		if d.IsDir() && excluded[d.Name()] {
			return filepath.SkipDir
		}

		// Look for .tf files
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tf") {
			found, err := scanTerraformFile(path)
			if err != nil {
				return err
			}
			modules = append(modules, found...)
		}
		return nil
	})

	if err != nil {
		fmt.Println("Error walking directories:", err)
		return
	}

	if len(modules) == 0 {
		fmt.Println("No Terraform modules found.")
		return
	}

	printTable(modules)
}

func scanTerraformFile(path string) ([]ModuleInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := string(content)

	moduleBlockRegex := regexp.MustCompile(`module\s+"([^"]+)"\s*{([^}]+)}`)
	sourceRegex := regexp.MustCompile(`source\s*=\s*"([^"]+)"`)

	var results []ModuleInfo

	matches := moduleBlockRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		moduleName := match[1]
		blockBody := match[2]

		sourceMatch := sourceRegex.FindStringSubmatch(blockBody)
		if len(sourceMatch) > 1 {
			source := sourceMatch[1]
			if strings.Contains(source, "github.com") {
				gitRepo, version := parseGitHubSource(source)
				moduleNameFromRepo := extractModuleNameFromGitRepo(gitRepo)
				results = append(results, ModuleInfo{
					ResourceName: moduleName,
					ModuleName:   moduleNameFromRepo,
					GitHubRepo:   gitRepo,
					Version:      version,
				})
			}
		}
	}

	return results, nil
}

// Parse GitHub URL to extract the repo and ref (branch/tag)
func parseGitHubSource(source string) (string, string) {
	source = strings.TrimPrefix(source, "git::")
	parts := strings.Split(source, "?")
	baseURL := parts[0]
	ref := ""

	if len(parts) > 1 {
		query := parts[1]
		for _, kv := range strings.Split(query, "&") {
			if strings.HasPrefix(kv, "ref=") {
				ref = strings.TrimPrefix(kv, "ref=")
			}
		}
	}

	baseURL = strings.TrimSuffix(baseURL, ".git")
	return baseURL, ref
}

// Extract module name from the GitHub repo URL (after github.com/ and before .git)
func extractModuleNameFromGitRepo(gitRepo string) string {
	parts := strings.Split(gitRepo, "/")
	if len(parts) > 1 {
		// Take the part after github.com/
		return strings.Join(parts[1:], "/")
	}
	return gitRepo // fallback if something unexpected happens
}

func printTable(modules []ModuleInfo) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Resource Name", "Module Name", "GitHub Repo", "Version"})

	for _, m := range modules {
		table.Append([]string{m.ResourceName, m.ModuleName, m.GitHubRepo, m.Version})
	}

	table.Render()
}

