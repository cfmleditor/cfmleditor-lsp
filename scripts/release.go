//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fatal("Usage: go run scripts/release.go [--dry-run] <version>\n  e.g. go run scripts/release.go 0.1.11")
	}

	dryRun := false
	args := os.Args[1:]
	if args[0] == "--dry-run" {
		dryRun = true
		args = args[1:]
	}
	if len(args) == 0 {
		fatal("Usage: go run scripts/release.go [--dry-run] <version>")
	}
	version := args[0]
	validateVersion(version)

	// Check for uncommitted changes
	out := git("status", "--porcelain")
	if strings.TrimSpace(out) != "" {
		fatal("working directory has uncommitted changes")
	}

	// Check gh CLI is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		fmt.Println("Not authenticated with GitHub. Running gh auth login...")
		run("gh", "auth", "login")
	}
	// Ensure git uses gh credentials
	run("gh", "auth", "setup-git")

	// Check tag doesn't exist
	tag := "v" + version
	out = git("tag", "-l", tag)
	if strings.TrimSpace(out) != "" {
		fatal("tag %s already exists", tag)
	}

	// Check version is greater than current
	current := strings.TrimSpace(readFile("VERSION"))
	if !versionGreater(version, current) {
		fatal("version %s is not greater than current %s", version, current)
	}

	// Fetch and check not behind remote
	fmt.Println("Fetching from remote...")
	gitExec("fetch")
	out = git("rev-list", "--count", "HEAD..@{u}")
	if strings.TrimSpace(out) != "0" {
		fatal("local branch is %s commit(s) behind remote", strings.TrimSpace(out))
	}

	// Update CHANGELOG.md
	changelog := readFile("CHANGELOG.md")
	unreleased := "## [Unreleased]"
	if !strings.Contains(changelog, unreleased) {
		fatal("CHANGELOG.md missing ## [Unreleased] section")
	}
	parts := strings.SplitN(changelog, unreleased, 2)
	after := parts[1]
	// Find content between [Unreleased] and next ## heading
	content := after
	if idx := strings.Index(after[1:], "\n## ["); idx >= 0 {
		content = after[:idx+1]
	}
	if strings.TrimSpace(content) == "" {
		fatal("CHANGELOG.md has no content under ## [Unreleased]")
	}

	if dryRun {
		fmt.Println("\n[dry-run] All checks passed. Would:")
		fmt.Printf("  - Update CHANGELOG.md: move unreleased to [%s]\n", version)
		fmt.Printf("  - Update VERSION to %s\n", version)
		fmt.Println("  - Run: make build")
		fmt.Println("  - Run: go test ./...")
		fmt.Println("  - Run: make lint")
		fmt.Printf("  - Commit, tag %s, push\n", tag)
		return
	}

	fmt.Printf("\nAbout to release %s. This will commit, tag, and push to origin.\n", tag)
	fmt.Print("Continue? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Aborted.")
		os.Exit(0)
	}

	newChangelog := strings.Replace(changelog, unreleased, unreleased+"\n\n## ["+version+"]", 1)
	writeFile("CHANGELOG.md", newChangelog)
	fmt.Printf("Updated CHANGELOG.md: moved unreleased to [%s]\n", version)

	// Update VERSION file
	writeFile("VERSION", version+"\n")
	fmt.Printf("Updated VERSION to %s\n", version)

	// Build
	fmt.Println("\nBuilding...")
	run("make", "build")

	// Test
	fmt.Println("Testing...")
	run("go", "test", "./...")

	// Lint
	fmt.Println("Linting...")
	run("make", "lint")

	// Commit, tag, push
	fmt.Println("\nCommitting...")
	gitExec("add", "VERSION", "CHANGELOG.md")
	gitExec("commit", "-m", "Release "+tag)

	fmt.Printf("Tagging %s...\n", tag)
	gitExec("tag", tag)

	fmt.Println("Pushing...")
	gitExec("push")
	gitExec("push", "origin", tag)

	fmt.Printf("\nReleased %s\n", tag)
}

func validateVersion(v string) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		fatal("version must be in format X.Y.Z (e.g. 0.1.11)")
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			fatal("version must be in format X.Y.Z (e.g. 0.1.11)")
		}
	}
}

func versionGreater(a, b string) bool {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na > nb {
			return true
		}
		if na < nb {
			return false
		}
	}
	return false
}

func git(args ...string) string {
	cmd := exec.Command("git", args...)
	out, _ := cmd.Output()
	return string(out)
}

func gitExec(args ...string) {
	run("git", args...)
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("command failed: %s %s", name, strings.Join(args, " "))
	}
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("failed to read %s: %v", path, err)
	}
	return string(data)
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fatal("failed to write %s: %v", path, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
