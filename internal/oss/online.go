package oss

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

// repoView is what the forge is asked about the repository itself.
type repoView struct {
	Description      string                  `json:"description"`
	LicenseInfo      *struct{ Key string }   `json:"licenseInfo"`
	Visibility       string                  `json:"visibility"`
	DefaultBranchRef *struct{ Name string }  `json:"defaultBranchRef"`
	RepositoryTopics []struct{ Name string } `json:"repositoryTopics"`
	HasIssuesEnabled bool                    `json:"hasIssuesEnabled"`
}

func (l *Linter) ghView() (*repoView, string) {
	if !lint.Have("gh") {
		return nil, "gh is not installed"
	}
	slug := l.slug()
	if slug == "" {
		return nil, "no origin remote to ask about"
	}
	out, ok := l.repo.Run("gh", "repo", "view", slug, "--json",
		"description,licenseInfo,visibility,defaultBranchRef,repositoryTopics,hasIssuesEnabled")
	if !ok {
		return nil, lint.Truncate(lastLine(out), 120)
	}
	var view repoView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		return nil, "gh returned something that is not JSON"
	}
	return &view, ""
}

var remoteVersionTag = regexp.MustCompile(`^v?\d+\.\d+\.\d+[\w.+-]*$`)

var onlineRules = []rule{{
	id: "OSS-801", severity: lint.Error, online: true,
	title: "The repository has a description and a detected licence",
	why: "The description is what a search result and every list of repositories shows. " +
		"An empty one makes the project look abandoned before it is read.",
	check: func(l *Linter) []lint.Problem {
		view, why := l.ghView()
		if view == nil {
			return []lint.Problem{lint.Skipf("OSS-801", "%s", why)}
		}
		var out []lint.Problem
		if strings.TrimSpace(view.Description) == "" {
			out = append(out, lint.Errorf("OSS-801", "the repository has no description"))
		}
		if view.LicenseInfo == nil {
			out = append(out, lint.Errorf("OSS-801", "GitHub does not detect a licence"))
		}
		if !view.HasIssuesEnabled {
			out = append(out, lint.Warnf("OSS-801", "issues are disabled, so nobody can report a bug"))
		}
		if view.DefaultBranchRef != nil && view.DefaultBranchRef.Name != "" &&
			view.DefaultBranchRef.Name != "main" {
			out = append(out, lint.Warnf("OSS-801", "the default branch is %s", view.DefaultBranchRef.Name))
		}
		if len(view.RepositoryTopics) == 0 {
			out = append(out, lint.Warnf("OSS-801", "no topics, so the repository appears in no topic listing"))
		}
		// Offered on a public repository only, so this is a publication step
		// rather than something that could have been done earlier.
		if view.Visibility == "PUBLIC" {
			body, ok := l.repo.Run("gh", "api", "repos/"+l.slug()+"/private-vulnerability-reporting")
			if ok && strings.Contains(strings.ReplaceAll(body, " ", ""), `"enabled":false`) {
				out = append(out, lint.Errorf("OSS-801",
					"private vulnerability reporting is off, and CONTRIBUTING.md points a "+
						"reporter at it. Enable it with `gh api -X PUT repos/<owner>/<repo>/private-vulnerability-reporting`"))
			}
		}
		return out
	},
}, {
	id: "OSS-802", severity: lint.Error, online: true,
	title: "No private ref was pushed",
	why: "What a clone can reach is the set of refs on the remote, and every ref drags its " +
		"whole history with it. A backup branch publishes exactly what a rewrite removed, " +
		"and a leftover tag pins the old history whatever `main` is squashed to. A " +
		"force-push touches neither.",
	check: func(l *Linter) []lint.Problem {
		listing, err := l.repo.Git("ls-remote", "--heads", "--tags", "origin")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-802", "cannot reach the remote")}
		}
		var out []lint.Problem
		var branches []string
		tags := map[string]bool{}
		for line := range strings.SplitSeq(listing, "\n") {
			if _, after, ok := strings.Cut(line, "refs/heads/"); ok {
				branches = append(branches, after)
			}
			if _, after, ok := strings.Cut(line, "refs/tags/"); ok {
				tags[strings.TrimSuffix(after, "^{}")] = true
			}
		}
		var suspect, others []string
		for _, b := range branches {
			switch {
			case privateBranch.MatchString(b):
				suspect = append(suspect, b)
			case b != "main":
				others = append(others, b)
			}
		}
		if len(others) > 0 {
			out = append(out, lint.Warnf("OSS-802",
				"%d branch(es) on the remote besides main, each published with the repository: %s",
				len(others), strings.Join(lint.First(others, 8), ", ")))
		}
		if len(suspect) > 0 {
			out = append(out, lint.Errorf("OSS-802",
				"branches on the remote that must be deleted: %s. Use `git push origin --delete <branch>`",
				strings.Join(lint.First(suspect, 8), ", ")))
		}
		var stale []string
		for _, t := range lint.SortedKeys(tags) {
			if !remoteVersionTag.MatchString(t) {
				stale = append(stale, t)
			}
		}
		if len(stale) > 0 {
			out = append(out, lint.Errorf("OSS-802",
				"tags on the remote that are not versions, each pinning the history behind "+
					"it: %s. Use `git push origin :refs/tags/<tag>`", strings.Join(lint.First(stale, 6), ", ")))
		}
		return out
	},
}, {
	id: "OSS-803", severity: lint.Warn,
	title: "Only the publishing remote is configured",
	why: "A second remote pointing at a personal machine is how a push meant for the forge " +
		"lands somewhere else, and its name is a fact about your network.",
	check: func(l *Linter) []lint.Problem {
		listing, err := l.repo.Git("remote")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-803", "no git repository")}
		}
		var extra []string
		for r := range strings.SplitSeq(listing, "\n") {
			if r = strings.TrimSpace(r); r != "" && r != "origin" {
				extra = append(extra, r)
			}
		}
		if len(extra) > 0 {
			return []lint.Problem{lint.Warnf("OSS-803", "remotes other than origin: %s",
				strings.Join(extra, ", "))}
		}
		return nil
	},
}}

// rules is every readiness rule, in the order the families are numbered to be
// read.
var rules = concat(licenceRules, documentRules, leakRules, buildRules,
	cloneRules, ledgerRules, historyRules, onlineRules)

func concat(groups ...[]rule) []rule {
	var out []rule
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
