package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/erauner/homelab-todoist-declarative/internal/reconcile"
)

type Options struct {
	JSON bool
}

func PrintPlan(w io.Writer, plan *reconcile.Plan, opts Options) error {
	if opts.JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	fmt.Fprintln(w, "Plan:")
	if plan == nil || len(plan.Operations) == 0 {
		fmt.Fprintln(w, "  No changes.")
		return nil
	}

	// Group by kind for readability while maintaining deterministic order.
	currentKind := reconcile.Kind("")
	for _, op := range plan.Operations {
		if op.Kind != currentKind {
			currentKind = op.Kind
			fmt.Fprintf(w, "\n  %s:\n", kindHeading(currentKind))
		}
		sym := symbol(op.Action)
		fmt.Fprintf(w, "    %s %s %q", sym, op.Action, op.Name)
		if len(op.Changes) > 0 {
			fmt.Fprintln(w, ":")
			for _, ch := range op.Changes {
				fmt.Fprintf(w, "      - %s: %s -> %s\n", ch.Field, ch.From, ch.To)
			}
		} else {
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d to create, %d to update, %d to move, %d to delete, %d to reorder.\n",
		plan.Summary.Create, plan.Summary.Update, plan.Summary.Move, plan.Summary.Delete, plan.Summary.Reorder)

	if len(plan.Notes) > 0 {
		fmt.Fprintln(w, "Notes:")
		for _, n := range plan.Notes {
			fmt.Fprintf(w, "  - %s\n", n)
		}
	}

	return nil
}

func PrintApplyResult(w io.Writer, res *reconcile.ApplyResult, opts Options) error {
	if opts.JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if res == nil {
		fmt.Fprintln(w, "No results.")
		return nil
	}
	if len(res.Applied) == 0 {
		fmt.Fprintln(w, "No changes.")
		return nil
	}
	fmt.Fprintln(w, "Applied:")
	for _, r := range res.Applied {
		sym := symbol(r.Action)
		if r.Status == "ok" {
			fmt.Fprintf(w, "  %s %s %s %q\n", sym, r.Kind, r.Action, r.Name)
		} else {
			fmt.Fprintf(w, "  %s %s %s %q: %s\n", sym, r.Kind, r.Action, r.Name, r.Status)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d to create, %d to update, %d to move, %d to delete, %d to reorder.\n",
		res.Summary.Create, res.Summary.Update, res.Summary.Move, res.Summary.Delete, res.Summary.Reorder)
	return nil
}

func symbol(action reconcile.Action) string {
	switch action {
	case reconcile.ActionCreate:
		return "+"
	case reconcile.ActionDelete:
		return "-"
	default:
		return "~"
	}
}

func kindHeading(k reconcile.Kind) string {
	switch k {
	case reconcile.KindProject:
		return "Projects"
	case reconcile.KindLabel:
		return "Labels"
	case reconcile.KindFilter:
		return "Filters"
	case reconcile.KindTask:
		return "Tasks"
	default:
		s := string(k)
		if s == "" {
			return "Unknown"
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
}
