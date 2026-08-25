package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/oss"
	"github.com/codesweep-ai/lint/internal/prose"
	"github.com/codesweep-ai/lint/internal/refs"
	"github.com/codesweep-ai/lint/internal/surface"
)

// split maps each rule the claims linter carried to the rule that replaced it.
//
// The numbers moved because the families did. Each new command numbers its own
// families from one, so a reader of `--explain` meets a contiguous list rather
// than the holes that preserving the old trailing digits would have left.
var split = map[string]string{
	"WALK-101": "SURF-101",
	"WALK-102": "SURF-102",
	"WALK-103": "SURF-103",
	"WALK-104": "SURF-104",
	"WALK-201": "SURF-201",
	"WALK-202": "SURF-202",
	"WALK-401": "SURF-301",
	"WALK-402": "SURF-302",
	"WALK-403": "SURF-303",
	"WALK-302": "REF-101",
	"WALK-303": "REF-102",
	"WALK-304": "REF-103",
	"WALK-301": "REF-201",
	"WALK-501": "REF-202",
	"WALK-601": "REF-301",
	"WALK-602": "REF-302",
	"WALK-603": "REF-303",
}

// renamed returns the rule that replaced the identifier given, and whether
// there is one. The prose rules kept their numbers and changed their prefix,
// so those are answered by the prefix rather than by a table of thirteen
// entries that says the same thing.
func renamed(id string) (string, bool) {
	if rest, ok := strings.CutPrefix(id, "DOC-"); ok {
		return "PROSE-" + rest, carried()["PROSE-"+rest]
	}
	to, ok := split[id]
	return to, ok
}

// carried is every rule identifier cs-lint answers for, whichever linter holds
// it.
func carried() map[string]bool {
	out := map[string]bool{}
	for _, r := range prose.Rules {
		out[r.ID] = true
	}
	for _, set := range [][]lint.RuleDoc{refs.Explain(), surface.Explain(), oss.Explain()} {
		for _, r := range set {
			out[r.ID] = true
		}
	}
	return out
}

// waiverBlock is the section of the tuning file that waives a rule, or empty
// where nothing does. The prose rules take no waiver: a writing rule is
// answered by editing the sentence.
func waiverBlock(id string) string {
	switch {
	case strings.HasPrefix(id, "REF-"):
		return "docs.refs.allow"
	case strings.HasPrefix(id, "SURF-"):
		return "docs.surface.allow"
	case strings.HasPrefix(id, "OSS-"):
		return "oss.allow"
	}
	return ""
}

// checkAllow reports a waiver naming a rule the linter it sits under does not
// carry.
//
// It stops the run rather than printing a warning. A waiver whose identifier
// matches nothing is a rule switched off in private, which is exactly what
// requiring a reason exists to prevent, and a warning about it would be read
// by nobody: the file is edited once and then never opened again.
func checkAllow(cfg *config.Config) error {
	for _, block := range []struct {
		where string
		allow map[string]string
		rules []lint.RuleDoc
	}{
		{"docs.refs.allow", cfg.Docs.Refs.Allow, refs.Explain()},
		{"docs.surface.allow", cfg.Docs.Surface.Allow, surface.Explain()},
		{"oss.allow", cfg.OSS.Allow, oss.Explain()},
	} {
		mine := map[string]bool{}
		for _, r := range block.rules {
			mine[r.ID] = true
		}
		for _, id := range lint.SortedKeys(block.allow) {
			if mine[id] {
				continue
			}
			return explainWaiver(block.where, id)
		}
	}
	return nil
}

// explainWaiver says what is wrong with one waiver: a rule that has been
// renamed, one that belongs under another section, or one that never existed.
func explainWaiver(where, id string) error {
	if to, ok := renamed(id); ok {
		switch block := waiverBlock(to); {
		case block == where:
			return fmt.Errorf("%s: %s is now %s. Rename it here, keeping the reason",
				where, id, to)
		case block != "":
			return fmt.Errorf("%s: %s is now %s. Waive it under %s, with the same reason",
				where, id, to, block)
		}
		return fmt.Errorf("%s: %s is now %s, and the prose rules take no waiver. "+
			"Edit the sentence, or narrow the knob that reported it", where, id, to)
	}
	if carried()[id] {
		return fmt.Errorf("%s: %s is not a rule this linter carries. Waive it under %s",
			where, id, waiverBlock(id))
	}
	return fmt.Errorf("%s: %s is not a rule cs-lint carries. "+
		"Run `cs-lint <linter> --explain` for the list it does", where, id)
}

// walkthroughCmd answers the command that was split, by name.
//
// It is hidden, so it is absent from the help tree and from every listing, and
// it exits 2 rather than 1: a gate still calling it is broken rather than
// failing. There are no releases and one fork, so this is the whole of the
// compatibility this change ships.
func walkthroughCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "walkthrough",
		Aliases:            []string{"walk"},
		Hidden:             true,
		Args:               cobra.ArbitraryArgs,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(*cobra.Command, []string) error {
			return errors.New("`walkthrough` was split in two. " +
				"`cs-lint surface` checks that the documented interface is the real one, " +
				"and `cs-lint refs` checks that every reference resolves. " +
				"The prose linter was `cs-lint docs` and is now `cs-lint prose`")
		},
	}
}
