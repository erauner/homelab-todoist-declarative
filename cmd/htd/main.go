package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/erauner/homelab-todoist-declarative/internal/config"
	"github.com/erauner/homelab-todoist-declarative/internal/output"
	"github.com/erauner/homelab-todoist-declarative/internal/reconcile"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/auth"
	todoisthttp "github.com/erauner/homelab-todoist-declarative/internal/todoist/http"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/sync"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/v1"
)

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		var ec ExitCodeError
		if errors.As(err, &ec) {
			if ec.Err != nil {
				fmt.Fprintln(os.Stderr, ec.Err.Error())
			}
			os.Exit(ec.Code)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

type ExitCodeError struct {
	Code int
	Err  error
}

func (e ExitCodeError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func newRootCmd() *cobra.Command {
	var (
		file    string
		jsonOut bool
		prune   bool
		verbose bool
		yes     bool
	)

	root := &cobra.Command{
		Use:   "htd",
		Short: "Homelab Todoist Declarative reconciler",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&file, "file", "f", config.DefaultPath(), "config file path")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "output JSON")
	root.PersistentFlags().BoolVar(&prune, "prune", false, "allow deletions (also gated by spec.prune.*)")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose debug logging")

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate config file (no network)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Load(file); err != nil {
				return err
			}
			// Keep output minimal; primary use is a smoke check in CI.
			if jsonOut {
				fmt.Fprintln(cmd.OutOrStdout(), `{"valid":true}`)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "OK")
			}
			return nil
		},
	}

	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Compute and print the plan (no mutations)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			cfg, err := config.Load(file)
			if err != nil {
				return err
			}

			token, source, err := auth.DiscoverToken()
			if err != nil {
				return err
			}

			logger := log.New(io.Discard, "", 0)
			if verbose {
				logger = log.New(cmd.ErrOrStderr(), "", log.LstdFlags)
			}
			_ = source

			httpClient := todoisthttp.New(token,
				todoisthttp.WithVerbose(verbose),
				todoisthttp.WithLogger(logger),
			)
			v1c := v1.New(httpClient)
			syncC := sync.New(httpClient)

			snap, err := reconcile.FetchSnapshot(ctx, v1c, syncC)
			if err != nil {
				return err
			}
			plan, err := reconcile.BuildPlan(cfg, snap, reconcile.Options{Prune: prune})
			if err != nil {
				return err
			}
			if err := output.PrintPlan(cmd.OutOrStdout(), plan, output.Options{JSON: jsonOut}); err != nil {
				return err
			}
			if plan.Summary.TotalChanges() > 0 {
				return ExitCodeError{Code: 2, Err: nil}
			}
			return nil
		},
	}

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the plan (mutating)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			cfg, err := config.Load(file)
			if err != nil {
				return err
			}
			token, _, err := auth.DiscoverToken()
			if err != nil {
				return err
			}
			logger := log.New(io.Discard, "", 0)
			if verbose {
				logger = log.New(cmd.ErrOrStderr(), "", log.LstdFlags)
			}
			httpClient := todoisthttp.New(token,
				todoisthttp.WithVerbose(verbose),
				todoisthttp.WithLogger(logger),
			)
			v1c := v1.New(httpClient)
			syncC := sync.New(httpClient)

			snap, err := reconcile.FetchSnapshot(ctx, v1c, syncC)
			if err != nil {
				return err
			}
			plan, err := reconcile.BuildPlan(cfg, snap, reconcile.Options{Prune: prune})
			if err != nil {
				return err
			}
			if err := output.PrintPlan(cmd.OutOrStdout(), plan, output.Options{JSON: jsonOut}); err != nil {
				return err
			}
			if plan.Summary.TotalChanges() == 0 {
				return nil
			}

			if !yes {
				ok, err := confirmApply(cmd.InOrStdin(), cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if !ok {
					return ExitCodeError{Code: 1, Err: fmt.Errorf("aborted")}
				}
			}

			res, err := reconcile.Apply(ctx, cfg, snap, plan, reconcile.Clients{V1: v1c, Sync: syncC}, reconcile.Options{Prune: prune})
			if err != nil {
				return err
			}
			if err := output.PrintApplyResult(cmd.OutOrStdout(), res, output.Options{JSON: jsonOut}); err != nil {
				return err
			}
			return nil
		},
	}
	applyCmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")

	root.AddCommand(validateCmd)
	root.AddCommand(planCmd)
	root.AddCommand(applyCmd)

	return root
}

func confirmApply(in io.Reader, errOut io.Writer) (bool, error) {
	fmt.Fprint(errOut, "Apply these changes? [y/N]: ")
	var resp string
	if _, err := fmt.Fscanln(in, &resp); err != nil {
		// If user hits enter without typing, Fscanln returns error (unexpected newline).
		// Treat as no.
		return false, nil
	}
	s := strings.TrimSpace(strings.ToLower(resp))
	return s == "y" || s == "yes", nil
}
