package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	c "github.com/gookit/color"
	"github.com/katbyte/gogo-jira-stats/lib/cache"
	"github.com/spf13/cobra"
)

func CmdGraphs(_ *cobra.Command, args []string) error {
	var err error
	f := GetFlags()

	// todo add to flags
	outPath := "graphs"

	// ensure path exists
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		err := os.MkdirAll(outPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create path: %w", err)
		}
	}

	// default to past year
	from := time.Now().AddDate(-1, 0, 0)
	from = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
	to := time.Now()

	aragc := len(args)
	if aragc > 1 {
		from, err = time.Parse("2006-01", args[0])
		if err != nil {
			return fmt.Errorf("failed to parse time %s : %w", args[0], err)
		}

		if aragc == 2 {
			to, err = time.Parse("2006-01", args[1])
			if err != nil {
				return fmt.Errorf("failed to parse time %s : %w", args[1], err)
			}
		}
	}

	// open cache
	cache, err := cache.Open(f.CachePath)
	if err != nil {
		return fmt.Errorf("opening cache %s: %w", f.CachePath, err)
	}
	defer cache.DB.Close()

	c.Printf("Generating graphs for issues from <white>%s</> to <white>%s</>...\n", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err = GraphRepoOpenIssuesDaily(cache, outPath, from, to); err != nil {
		return fmt.Errorf("failed to generate daily open pr graphs path: %w", err)
	}
	return nil
}

type DailyOpenIssues struct {
	Date     time.Time
	Total    int
	Statuses map[string]int // jira status are freeform, so lets just allow any
}

func GraphRepoOpenIssuesDaily(theCache *cache.Cache, outPath string, from, to time.Time) error {
	// f := GetFlags() // todo out path ends up in flags

	c.Printf("    PRs open daily..\n")

	// get all open prs for range (this should be opened no created but currently we don't have closed date in the cache or easily accessible
	/*prs, err := theCache.GetIssuesCreatedInDateRange(from, to)
	if err != nil {
		return fmt.Errorf("getting PRs: %w", err)
	}*/

	// for now lets just go over ALL issues until we can query for any open within a date
	issues, err := theCache.GetAllIssues()
	if err != nil {
		return fmt.Errorf("getting all issuess: %w", err)

	}

	// todo sort these in a predetermined order
	allStatuses := []string{"Other", "Pending Triage", "Icebox", "Blocked", "Needs More Info", "Accepted", "Prioritized", "In Progress", "In Review"}
	statusLookupMap := map[string]bool{}
	for _, status := range allStatuses {
		statusLookupMap[status] = true
	}

	// populate dates
	dates := map[string]DailyOpenIssues{}
	for day := from.AddDate(0, 0, -1); day.Before(to.AddDate(0, 0, 1)); day = day.AddDate(0, 0, 1) {
		k := day.Format("2006-01-02")
		dates[k] = DailyOpenIssues{
			Date:     day,
			Statuses: map[string]int{},
		}

		for _, status := range allStatuses {
			dates[k].Statuses[status] = 0
		}

	}

	for _, i := range *issues {
		opened := time.Date(i.Created.Year(), i.Created.Month(), i.Created.Day(), 0, 0, 0, 0, time.UTC)

		if opened.After(to) {
			continue
		}

		// figure out timeline of events that matter
		events, err := theCache.GetIssueEventsForField(i.Key, "status")
		if err != nil {
			return fmt.Errorf("getting events for %s: %w", i.Key, err)
		}

		if i.Key == "IPL-5530" {
			fmt.Printf("found IPL-5530\n")
		}

		// for each day from open to closed (or now) count this PR using the above array to figure out its "state"
		// by playing back events to "set the state" until the events
		status := "Pending Triage"
		eventIndex := 0
		for day := opened; ; day = day.AddDate(0, 0, 1) {

			// go through all events for "today" and set the status
			if len(events) > 0 {
				for ; eventIndex < len(events) && events[eventIndex].Date.Before(day.AddDate(0, 0, 1)); eventIndex++ {

					status = events[eventIndex].To
					if status == "Closed" {
						break
					}

					if ok, _ := statusLookupMap[status]; !ok {
						status = "Other"
					}
				}
			}

			// skip data before the "from" date, but here so status is updated
			if day.Before(from.AddDate(0, 0, -1)) {
				continue
			}

			k := day.Format("2006-01-02")
			dayData := dates[k]
			dayData.Total++
			dayData.Statuses[status]++
			dates[k] = dayData

			if day.After(to.AddDate(0, 0, -1)) {
				break
			}

			// if closed we're done
			if status == "Closed" {
				break
			}
		}

		if status == "Pending Triage" {
			fmt.Printf("unhandled status: %s\n", i.Key)
		}
	}

	var xAxis []string

	lineDataMap := map[string][]opts.LineData{}
	for _, status := range allStatuses {
		lineDataMap[status] = []opts.LineData{}
	}

	// todo save csv file?
	days := make([]string, 0, len(dates))
	for day := range dates {
		days = append(days, day)
	}
	sort.Strings(days)

	for _, date := range days {
		day := dates[date]

		if day.Date.Before(from.AddDate(0, 0, -1)) {
			continue
		}

		if day.Date.After(to) {
			continue
		}

		xAxis = append(xAxis, date)
		for _, status := range allStatuses {
			lineDataMap[status] = append(lineDataMap[status], opts.LineData{Value: day.Statuses[status]})
		}
	}

	// render graph
	graph := charts.NewLine()
	graph.SetGlobalOptions(
		// charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Azure Team JIRAs Open (daily)",
			Subtitle: "By Status: " + strings.Join(allStatuses, ", "),
			Left:     "center", // nolint:misspell
		}),

		charts.WithXAxisOpts(opts.XAxis{
			Name: "Date",
			// AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} x-unit"},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "# Issues",
			// AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} x-unit"},
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1500px",
			Height: "750px",
		}),
		charts.WithColorsOpts(opts.Colors{
			"#440154", // Deep Violet
			"#7B414B", // Dark Red, replacing the burnt orange
			"#365C8D", // Dark Blue
			"#46337E", // Violet
			"#006D5B", // Dark Teal, replacing the pale blue
			"#277F8E", // Blue-Green
			"#1FA187", // Aquamarine
			"#4AC16D", // Light Green
			"#9FDA3A", // Yellowish Green
		}),

		charts.WithToolboxOpts(opts.Toolbox{Show: true}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:      true,
			Trigger:   "axis",
			TriggerOn: "mousemove",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: true,
			Top:  "bottom",
			Left: "center", // nolint:misspell
		}),
	)

	// Put data into instance
	graph.SetXAxis(xAxis)

	prStackOps := []charts.SeriesOpts{
		charts.WithAreaStyleOpts(opts.AreaStyle{Opacity: 0.8}),
		charts.WithLineChartOpts(opts.LineChart{Stack: "prs"}),
		charts.WithLineStyleOpts(opts.LineStyle{Width: 1, Opacity: 0.9}),
	}

	for _, status := range allStatuses {
		graph.AddSeries(status, lineDataMap[status]).SetSeriesOptions(prStackOps...)
	}

	// Where the magic happens
	file, err := os.Create(outPath + "/daily-issues-open.html")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	err = graph.Render(file)
	if err != nil {
		return fmt.Errorf("failed to render graph graph: %w", err)
	}

	return nil
}
