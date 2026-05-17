package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

// Persistent output flags, bound in root.go. They are agent-native conveniences:
// --json for parseable output, --dry-run for safe mutation previews, --limit to
// bound list responses.
var (
	jsonOutput bool
	dryRun     bool
	limitFlag  int
)

// applyLimit truncates items to limit entries, reporting whether truncation
// occurred. A limit <= 0 means unlimited.
func applyLimit[T any](items []T, limit int) ([]T, bool) {
	if limit <= 0 || len(items) <= limit {
		return items, false
	}
	return items[:limit], true
}

// emitJSON writes v to stdout as indented JSON.
func emitJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// emitResult reports the outcome of a single-resource command: the resource as
// JSON when --json is set, otherwise the human-readable line.
func emitResult(textLine string, v any) error {
	if jsonOutput {
		return emitJSON(v)
	}
	fmt.Println(textLine)
	return nil
}

// emitList renders a list command's results: a JSON array when --json is set,
// otherwise one printItem call per entry. Results are bounded by --limit, and a
// truncation hint is written to stderr so --json stdout stays a clean document.
func emitList[T any](items []T, printItem func(T)) error {
	limited, truncated := applyLimit(items, limitFlag)
	if jsonOutput {
		if err := emitJSON(limited); err != nil {
			return err
		}
	} else {
		for _, it := range limited {
			printItem(it)
		}
	}
	if truncated {
		fmt.Fprintf(os.Stderr,
			"showing %d of %d; pass --limit 0 for all, or filter to narrow\n",
			len(limited), len(items))
	}
	return nil
}

// emitDryRun reports a mutation that was validated but not sent. In --json mode
// it emits {"dry_run": true, "would": <request>}; otherwise a DRY RUN line.
func emitDryRun(textLine string, request any) error {
	if jsonOutput {
		return emitJSON(map[string]any{"dry_run": true, "would": request})
	}
	fmt.Println("DRY RUN (no changes sent): " + textLine)
	return nil
}
