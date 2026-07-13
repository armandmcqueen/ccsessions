package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/armandmcqueen/ccsessions/internal/config"
	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/repogroup"
	"github.com/spf13/cobra"
)

// auditRow is one distinct source directory being coalesced into a group.
type auditRow struct {
	Group    string `json:"group"`
	Reason   string `json:"reason"`
	CWD      string `json:"cwd"`
	Detail   string `json:"detail"`
	Sessions int    `json:"sessions"`
}

func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Show how sessions are coalesced into groups, and why",
		Long: "Audits the grouping decisions: for every group, shows which working " +
			"directories are folded into it, how many sessions each contributes, and the " +
			"reason. Groups where unrelated directories collide on a bare basename are " +
			"flagged with ⚠, so you can spot wrong merges.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := cfgFromCmd(cmd)
			if err != nil {
				return err
			}
			refs, err := discover.Sessions(cfg.ClaudeDir, cfg.Projects)
			if err != nil {
				return err
			}

			mode := cfg.GroupBy
			if mode == "" {
				mode = repogroup.ModeRepo
			}
			g := repogroup.New(mode)
			if cfg.GroupRules != "" {
				rs, err := repogroup.LoadRules(cfg.GroupRules)
				if err != nil {
					return err
				}
				g.SetRules(rs)
			}

			// Prime with every cwd first so deleted-dir sessions can merge with live
			// siblings — this mirrors what a bulk render does.
			cwds := make([]string, len(refs))
			for i, ref := range refs {
				cwds[i] = discover.PeekCWD(ref.MainPath)
				g.Prime(cwds[i])
			}

			// Aggregate sessions per (group, cwd).
			type key struct{ group, cwd string }
			agg := map[key]*auditRow{}
			for i, ref := range refs {
				grp, reason, detail := g.Explain(cwds[i], ref.ProjectKey)
				k := key{grp, cwds[i]}
				row, ok := agg[k]
				if !ok {
					row = &auditRow{Group: grp, Reason: reason, CWD: cwds[i], Detail: detail}
					agg[k] = row
				}
				row.Sessions++
			}

			rows := make([]*auditRow, 0, len(agg))
			for _, r := range agg {
				rows = append(rows, r)
			}

			dirsPerGroup := map[string]map[string]bool{}
			sessionsPerGroup := map[string]int{}
			fragileGroup := map[string]bool{}
			for _, r := range rows {
				if dirsPerGroup[r.Group] == nil {
					dirsPerGroup[r.Group] = map[string]bool{}
				}
				dirsPerGroup[r.Group][r.CWD] = true
				sessionsPerGroup[r.Group] += r.Sessions
				// A group is fragile when a session reached it through a guess: a
				// bare basename (collides across repos, splits subdirs off) or no cwd
				// at all. Explicit rule matches and git resolution are confident.
				if r.Reason == repogroup.ReasonBasenameFallback || r.Reason == repogroup.ReasonNoCWD {
					fragileGroup[r.Group] = true
				}
			}
			suspect := func(group string) bool { return fragileGroup[group] }

			// Sort: suspect groups first, then by session count desc, then group, cwd.
			sort.Slice(rows, func(i, j int) bool {
				ci, cj := suspect(rows[i].Group), suspect(rows[j].Group)
				if ci != cj {
					return ci
				}
				if si, sj := sessionsPerGroup[rows[i].Group], sessionsPerGroup[rows[j].Group]; si != sj {
					return si > sj
				}
				if rows[i].Group != rows[j].Group {
					return rows[i].Group < rows[j].Group
				}
				return rows[i].CWD < rows[j].CWD
			})

			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			out := cmd.OutOrStdout()
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "FLAG\tGROUP\tSESS\tREASON\tSOURCE DIRECTORY\tWHY")
			var suspectGroups int
			lastGroup := ""
			for _, r := range rows {
				flag := ""
				if suspect(r.Group) {
					flag = "⚠"
				}
				// Blank the repeated group/flag cells for readability.
				groupCell, flagCell := r.Group, flag
				if r.Group == lastGroup {
					groupCell, flagCell = "", ""
				} else if flag == "⚠" {
					suspectGroups++
				}
				lastGroup = r.Group
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n",
					flagCell, groupCell, r.Sessions, r.Reason, r.CWD, r.Detail)
			}
			tw.Flush()

			groups := len(dirsPerGroup)
			fmt.Fprintf(out, "\n%d sessions → %d groups", len(refs), groups)
			if mode == repogroup.ModeRepo && suspectGroups > 0 {
				fmt.Fprintf(out, "  (⚠ %d group(s) not resolved to a real repo — bare basenames or missing cwd; these are the wrong/fragile merges)", suspectGroups)
			}
			fmt.Fprintln(out)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "machine-readable JSON output")
	// audit only reads; these let you preview a grouping before deploying it.
	cmd.Flags().String("group-by", "repo", `grouping to audit: "repo" or "project"`)
	cmd.Flags().String("group-rules", "", "path to a JSON regex grouping-rules file to audit (env "+config.EnvGroupRules+")")
	return cmd
}
