package hook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/git"
	"github.com/Ksschkw/driftlock/internal/llm"
	"github.com/Ksschkw/driftlock/internal/llm/types"
	"github.com/Ksschkw/driftlock/internal/output"
	"github.com/Ksschkw/driftlock/internal/parser"
)

// StatusCheck runs a full‑repo check: for every documentation file, it gathers
// all function signatures from the mapped source files (HEAD) and asks the LLM
// whether the documentation covers them all. Results are printed to stderr.
func StatusCheck(ctx context.Context) error {
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	// Build doc map from all tracked files, not just staged
	docMap, err := buildStatusDocMap(cfg.DocMapping, root)
	if err != nil {
		return err
	}
	if len(docMap) == 0 {
		fmt.Fprint(os.Stderr, output.GreenStr("No documentation mappings found. Nothing to check.\n"))
		return nil
	}

	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	anyOutOfSync := false

	for docPath, sourceFiles := range docMap {
		var sigList []string
		for _, src := range sourceFiles {
			content, err := git.GetFileContentAtHEAD(src)
			if err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read %s: %v\n", src, err)))
				continue
			}
			sigs := parser.ExtractSignatures(src, content)
			for _, sig := range sigs {
				sigList = append(sigList, fmt.Sprintf("%s: %s", src, sig.Signature))
			}
		}

		if len(sigList) == 0 {
			fmt.Fprint(os.Stderr, output.GreenStr(fmt.Sprintf("driftlock: %s => no signatures found; skipping.\n", docPath)))
			continue
		}

		docFullPath := filepath.Join(root, docPath)
		docContent, err := os.ReadFile(docFullPath)
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			continue
		}

		sigText := strings.Join(sigList, "\n")
		ok, explanation, err := checkDocCoverage(ctx, provider, sigText, string(docContent))
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("driftlock: %s => LLM error: %v\n", docPath, err)))
			continue
		}

		if ok {
			fmt.Fprint(os.Stderr, output.GreenStr(fmt.Sprintf("driftlock: %s => TRUE %s\n", docPath, explanation)))
		} else {
			fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("driftlock: %s => FALSE %s\n", docPath, explanation)))
			anyOutOfSync = true
		}
	}

	if anyOutOfSync {
		return fmt.Errorf("documentation drift detected")
	}
	return nil
}

// checkDocCoverage sends a prompt asking whether the given signatures are covered by the doc.
func checkDocCoverage(ctx context.Context, provider types.Provider, signatures, doc string) (bool, string, error) {
	prompt := fmt.Sprintf(`You are a documentation auditor. Below is a list of function signatures from the source code and the current documentation file.

Signatures:
%s

Documentation:
%s

Does the documentation accurately cover all of these signatures? Answer exactly TRUE or FALSE, then a one‑sentence explanation.`, signatures, doc)
	return provider.Check(ctx, prompt, doc)
}

// buildStatusDocMap resolves doc_mapping entries to a map of doc path -> source files
// using all tracked files in the repository.
func buildStatusDocMap(entries []config.DocMapEntry, root string) (map[string][]string, error) {
	tracked, err := git.ListTrackedFiles()
	if err != nil {
		return nil, err
	}

	docMap := make(map[string][]string)
	for _, entry := range entries {
		for _, srcGlob := range entry.Sources {
			for _, file := range tracked {
				match, err := filepath.Match(srcGlob, file)
				if err != nil {
					return nil, err
				}
				if !match {
					if strings.HasPrefix(file, strings.TrimSuffix(srcGlob, "**")) {
						match = true
					}
				}
				if match {
					for _, doc := range entry.Docs {
						resolved, err := config.ResolveDocPath(doc, root)
						if err != nil {
							return nil, err
						}
						for _, rd := range resolved {
							docMap[rd] = append(docMap[rd], file)
						}
					}
				}
			}
		}
	}
	return docMap, nil
}
