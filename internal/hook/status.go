package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/git"
	"github.com/Ksschkw/driftlock/internal/output"
	"github.com/Ksschkw/driftlock/internal/parser"
)

// StatusOptions controls the repository-wide coverage report.
type StatusOptions struct {
	// Strict makes the command return an error when any public symbol is not
	// mentioned in its mapped documentation. The default is informational,
	// because a whole-repository scan of a mature project will always find
	// some, and a command that always fails is a command nobody runs.
	Strict bool
}

// StatusCheck reports, for each mapped document, which public symbols found in
// the tracked source files it does not mention.
//
// The check is deterministic: no model is consulted, so it is instant, free,
// and reproducible, and it can run in CI on every commit.
//
// The previous implementation built an auditor prompt and passed that string as
// the `diff` argument to the provider, which then rendered the check template
// around it. The model therefore received the coverage question nested inside
// "Structural changes:", with the document included twice, and answered under
// the wrong instructions.
func StatusCheck(opts StatusOptions) error {
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}
	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	tracked, err := git.ListTrackedFiles()
	if err != nil {
		return err
	}
	docMap, err := config.ResolveDocMapping(cfg.DocMapping, tracked, root)
	if err != nil {
		return err
	}
	if len(docMap) == 0 {
		fmt.Fprint(os.Stderr, output.GreenStr("driftlock: no documentation mappings found; nothing to report.\n"))
		return nil
	}

	docs := make([]string, 0, len(docMap))
	for doc := range docMap {
		docs = append(docs, doc)
	}
	sort.Strings(docs)

	totalSymbols, totalMissing := 0, 0
	for _, docPath := range docs {
		names := documentedSymbols(root, docMap[docPath])
		docBytes, err := os.ReadFile(filepath.Join(root, docPath))
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			continue
		}

		missing := missingSymbols(string(docBytes), names)
		totalSymbols += len(names)
		totalMissing += len(missing)

		covered := len(names) - len(missing)
		summary := fmt.Sprintf("driftlock: %s → %d/%d symbols documented", docPath, covered, len(names))
		switch {
		case len(names) == 0:
			fmt.Fprint(os.Stderr, output.YellowStr(summary+" (no signatures found; check doc_mapping and report_unparsed)\n"))
		case len(missing) == 0:
			fmt.Fprint(os.Stderr, output.GreenStr(summary+"\n"))
		default:
			shown := missing
			const limit = 20
			suffix := ""
			if len(shown) > limit {
				suffix = fmt.Sprintf(" (+%d more)", len(shown)-limit)
				shown = shown[:limit]
			}
			fmt.Fprint(os.Stderr, output.RedStr(summary+": missing "+strings.Join(shown, ", ")+suffix+"\n"))
		}
	}

	fmt.Fprintf(os.Stderr, "\ndriftlock: %d/%d public symbols are mentioned in their mapped documentation.\n",
		totalSymbols-totalMissing, totalSymbols)

	if opts.Strict && totalMissing > 0 {
		return fmt.Errorf("%d public symbol(s) are not documented", totalMissing)
	}
	return nil
}

// documentedSymbols returns the deduplicated, unqualified symbol names found in
// the given source files.
func documentedSymbols(root string, sources []string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, src := range sources {
		content, err := os.ReadFile(filepath.Join(root, src))
		if err != nil {
			continue
		}
		for _, sig := range parser.ExtractSignatures(src, string(content)) {
			name := bareName(sig.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// bareName strips a parser scope qualifier (`A.build` -> `build`). Coverage is
// about whether the documentation mentions the symbol, and documentation does
// not usually spell out the enclosing type.
func bareName(qualified string) string {
	if i := strings.LastIndexByte(qualified, '.'); i >= 0 {
		return qualified[i+1:]
	}
	return qualified
}

// missingSymbols returns the names that do not appear in the document,
// preserving the input order. Matching is case-insensitive.
func missingSymbols(doc string, names []string) []string {
	lower := strings.ToLower(doc)
	var missing []string
	for _, name := range names {
		if !strings.Contains(lower, strings.ToLower(name)) {
			missing = append(missing, name)
		}
	}
	return missing
}
