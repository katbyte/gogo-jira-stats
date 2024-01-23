package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
	c "github.com/gookit/color"
	"github.com/katbyte/gogo-jira-stats/lib/cache"
	"github.com/katbyte/gogo-jira-stats/lib/j"
	"github.com/spf13/cobra"
)

func CmdFetch(_ *cobra.Command, _ []string) error {
	f := GetFlags()

	// open cache
	cache, err := cache.Open(f.CachePath)
	if err != nil {
		return fmt.Errorf("opening cache %s: %w", f.CachePath, err)
	}

	i := j.NewInstance(f.Url, f.User, f.Token)

	c.Printf("Retrieving all issues matching <white>%s</> from <cyan>%s</>...\n", f.JQL, f.Url)
	c.Printf("  Fields %s\n", strings.Join(f.Fields, ", "))
	c.Printf("  Expand %s\n", strings.Join(f.Expand, ", "))

	n := 0
	err = i.ListAllIssues(f.JQL, &f.Fields, &f.Expand, func(results *models.IssueSearchScheme, resp *models.ResponseScheme) error {
		c.Printf("<magenta>%d</>-<lightMagenta>%d</> <darkGray>of %d</>\n", results.StartAt, results.MaxResults, results.Total)
		for _, i := range results.Issues {
			n++

			keyColour := "lightGreen"
			if i.Fields.Status.Name == "Closed" {
				keyColour = "green"
			}

			parsedDate, err := time.Parse("2006-01-02T15:04:05.000-0700", i.Fields.Created)
			if err != nil {
				return fmt.Errorf("failed to parse date %s: %w", i.Fields.Created, err)
			}

			c.Printf("<darkGray>%03d/%d</> <%s>%s</><darkGray>@%s</> - %s\n", n, results.Total, keyColour, i.Key, parsedDate.Format("2006-01-02"), i.Fields.Summary)
			if err = cache.UpsertIssueFromJIRA(i); err != nil {
				return fmt.Errorf("cache issue upsert failed: %w", err)
			}

			count, err := cache.UpsertEventsFromIssue(i)
			if err != nil {
				return fmt.Errorf("cache issue events upsert failed: %w", err)
			}
			c.Printf("    <darkGray>by </>%s, <%s>%s</> with <cyan>%d</> events\n", i.Fields.Creator.DisplayName, keyColour, i.Fields.Status.Name, *count)

			// if closed get events for status and find the date of the last one which is "closed" and update the issue with days open and "closed" date
			// todo, we don't care about this yet & given the low issue count ( < 1000, we can just parse all events for all issues when reporting and generating graphs)

		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to list issues for %s @ %s: %w", i.URL, f.JQL, err)
	}

	return nil
}
