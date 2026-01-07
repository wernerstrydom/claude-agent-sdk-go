# Repository Maintenance

Iterate through a list of repositories, clone them into temp directories, and ask Claude to check each one for common maintenance tasks. This is the kind of thing that compounds across many repos -- checking ten repositories by hand is tedious, but scripting it is straightforward.

## The Code

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

type RepoCheck struct {
	Repo    string   `json:"repo"`
	Issues  []string `json:"issues"`
	Summary string   `json:"summary"`
}

var repos = []string{
	"github.com/myorg/api-gateway",
	"github.com/myorg/auth-service",
	"github.com/myorg/billing",
	"github.com/myorg/dashboard",
	"github.com/myorg/notifications",
	"github.com/myorg/worker",
	"github.com/myorg/cli-tools",
	"github.com/myorg/shared-lib",
	"github.com/myorg/docs-site",
	"github.com/myorg/deploy-scripts",
}

const checkPrompt = `Analyze this repository and check for the following:

1. **Outdated dependencies**: Are there any dependencies that have major version updates available?
2. **Language version**: Is the project using a current, supported language version? (e.g. Go 1.21+ , Node 20+, Python 3.11+)
3. **Missing CI**: Does the project have CI configuration (GitHub Actions, etc.)? If so, is it current?
4. **Security**: Are there any obvious security issues (e.g. hardcoded secrets, missing .gitignore entries)?
5. **Missing capabilities**: Based on the dependencies already in use, are there common companion tools that should be added? (e.g. a Go project with HTTP handlers but no structured logging)

Be concise. List only actionable items.`

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	tmpRoot, err := os.MkdirTemp("", "repo-check-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpRoot)

	var results []RepoCheck

	for _, repo := range repos {
		repoDir := filepath.Join(tmpRoot, filepath.Base(repo))

		// Clone the repo
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1",
			fmt.Sprintf("https://%s.git", repo), repoDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("clone %s: %s", repo, out)
			continue
		}

		a, err := agent.New(ctx,
			agent.Model("claude-sonnet-4-5"),
			agent.WorkDir(repoDir),
			agent.MaxTurns(5),
		)
		if err != nil {
			log.Printf("%s: %v", repo, err)
			continue
		}

		var check RepoCheck
		check.Repo = repo

		result, err := a.Run(ctx, checkPrompt)
		a.Close()
		if err != nil {
			log.Printf("%s: %v", repo, err)
			continue
		}

		check.Summary = result.ResultText
		results = append(results, check)

		fmt.Printf("checked %s ($%.4f)\n", repo, result.CostUSD)
	}

	// Write report
	out, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile("repo-report.json", out, 0644)
	fmt.Printf("\n%d repositories checked, report written to repo-report.json\n", len(results))
}
```

## How It Works

1. Each repository is cloned with `--depth=1` into a temp directory (shallow clone to save time and disk).
2. An agent is created with `WorkDir` pointing at the cloned repo, so Claude can read the files.
3. `MaxTurns(5)` gives Claude enough turns to browse the project structure, read key files, and formulate a response.
4. Results are collected into a JSON report.
5. The temp directory is cleaned up when the program exits.

The point isn't that Claude gives you a perfect audit -- it's that you get a first pass across ten repos in one run instead of opening each one manually.

## Output

The `repo-report.json` file contains an entry per repo:

```json
[
  {
    "repo": "github.com/myorg/api-gateway",
    "issues": [],
    "summary": "Go 1.19 in go.mod, should be 1.21+. Missing golangci-lint config..."
  }
]
```

You can feed this into dashboards, Slack notifications, or issue trackers.

## Adapting This Pattern

The repo list and check prompt are the only moving parts. Other uses for the same structure:

- Check all repos for a specific vulnerability or deprecated API usage
- Verify all repos have consistent linting configuration
- Generate migration PRs across repos (e.g., update a shared dependency)
- Audit license compliance across an organization's repositories
