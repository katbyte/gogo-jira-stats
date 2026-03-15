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
	from := time.Now().AddDate(-2, 0, 0)
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

func colorizeStatus(status string) string {
	switch status {
	case "Closed":
		return c.Sprintf("<green>%s</>", status)
	case "In Progress", "In Development":
		return c.Sprintf("<lightBlue>%s</>", status)
	case "In Review":
		return c.Sprintf("<cyan>%s</>", status)
	case "Accepted", "To Do":
		return c.Sprintf("<lightGreen>%s</>", status)
	case "Prioritized":
		return c.Sprintf("<lightCyan>%s</>", status)
	case "Pending Triage":
		return c.Sprintf("<lightYellow>%s</>", status)
	case "Icebox":
		return c.Sprintf("<blue>%s</>", status)
	case "Blocked":
		return c.Sprintf("<red>%s</>", status)
	case "Needs More Info":
		return c.Sprintf("<magenta>%s</>", status)
	case "Other":
		return c.Sprintf("<yellow>%s</>", status)
	default:
		return c.Sprintf("<darkGray>%s</>", status)
	}
}

func GraphRepoOpenIssuesDaily(theCache *cache.Cache, outPath string, from, to time.Time) error {
	c.Printf("\n  📊 Issues open daily (stacked area)\n")

	// for now lets just go over ALL issues until we can query for any open within a date
	issues, err := theCache.GetAllIssues()
	if err != nil {
		return fmt.Errorf("getting all issues: %w", err)
	}
	c.Printf("    Loaded <white>%d</> issues from cache\n", len(*issues))

	// defined statuses for the graph (in stack order)
	allStatuses := []string{"Other", "Accepted", "Awaiting Prioritization", "Pending Triage", "Blocked", "Needs More Info", "To Do", "Prioritized", "In Progress", "In Review"}
	statusLookupMap := map[string]bool{}
	for _, status := range allStatuses {
		statusLookupMap[status] = true
	}

	// status mappings to normalize variations
	statusMappings := map[string]string{
		"In Development":             "In Progress",
		"Accepted":                   "To Do",
		"Need More Information":      "Needs More Info",
		"R&D to Investigate Further": "Awaiting Prioritization",
		"Security Triage":            "Pending Triage",
		"Under Review by R&D PM":     "In Review",
	}

	c.Printf("    Statuses: ")
	for i, s := range allStatuses {
		if i > 0 {
			c.Printf("<darkGray>, </>")
		}
		c.Printf("%s", colorizeStatus(s))
	}
	c.Printf("\n")
	c.Printf("    Mappings:\n")
	for from, to := range statusMappings {
		c.Printf("      <darkGray>%s</> → %s\n", from, colorizeStatus(to))
	}

	// scan all issues and their events to discover what statuses exist
	allFoundStatuses := map[string]int{}
	for _, i := range *issues {
		allFoundStatuses[i.Status]++
	}

	// also scan events to find historical statuses
	allEvents, err := theCache.GetAllEvents()
	if err != nil {
		return fmt.Errorf("getting all events: %w", err)
	}
	for _, e := range allEvents {
		if e.Field == "status" {
			allFoundStatuses[e.To]++
		}
	}

	c.Printf("    All statuses found (issues + events):\n")
	sortedFound := make([]string, 0, len(allFoundStatuses))
	for s := range allFoundStatuses {
		sortedFound = append(sortedFound, s)
	}
	sort.Strings(sortedFound)
	for _, s := range sortedFound {
		mapped := ""
		if m, ok := statusMappings[s]; ok {
			mapped = c.Sprintf(" <darkGray>→</> %s", colorizeStatus(m))
		} else if !statusLookupMap[s] && s != "Closed" {
			mapped = c.Sprintf(" <darkGray>→</> <yellow>Other</>")
		}
		c.Printf("      <darkGray>%4d</> %s%s\n", allFoundStatuses[s], colorizeStatus(s), mapped)
	}

	// populate dates
	days := int(to.Sub(from).Hours()/24) + 2
	c.Printf("    Generating data for <white>%d</> days (<white>%s</> to <white>%s</>)\n", days, from.Format("2006-01-02"), to.Format("2006-01-02"))

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

	// process each issue, replay events day by day
	c.Printf("    Processing <white>%d</> issues...\n", len(*issues))
	skippedAfterTo := 0
	closedCount := 0
	otherCount := 0

	for _, i := range *issues {
		opened := time.Date(i.Created.Year(), i.Created.Month(), i.Created.Day(), 0, 0, 0, 0, time.UTC)

		if opened.After(to) {
			skippedAfterTo++
			continue
		}

		// figure out timeline of events that matter
		events, err := theCache.GetIssueEventsForField(i.Key, "status")
		if err != nil {
			return fmt.Errorf("getting events for %s: %w", i.Key, err)
		}

		// figure out initial status before any events
		status := "Other"
		if len(events) == 0 {
			// no events, use current status (unless closed)
			if i.Status != "Closed" {
				status = i.Status
			}
		} else {
			// use the first event's From field as the initial status
			status = events[0].From
		}
		// apply mappings and fallback to Other
		if s, ok := statusMappings[status]; ok {
			status = s
		}
		if status != "Closed" && !statusLookupMap[status] {
			status = "Other"
		}

		// for each day from open to closed (or now) count this issue using the above array to figure out its "state"
		// by playing back events to "set the state" until the events
		eventIndex := 0
		for day := opened; ; day = day.AddDate(0, 0, 1) {

			// go through all events for "today" and set the status
			if len(events) > 0 {
				for ; eventIndex < len(events) && events[eventIndex].Date.Before(day.AddDate(0, 0, 1)); eventIndex++ {

					status = events[eventIndex].To
					if status == "Closed" {
						break
					}

					if s, ok := statusMappings[status]; ok {
						status = s
					}

					// any status not in our list goes to Other
					if !statusLookupMap[status] {
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
				closedCount++
				break
			}
		}

		if status == "Other" {
			otherCount++
		}
	}

	c.Printf("      <darkGray>Skipped %d issues created after range</>\n", skippedAfterTo)
	c.Printf("      <darkGray>%d issues closed within range</>\n", closedCount)
	if otherCount > 0 {
		c.Printf("      <yellow>%d</> issues with unmapped status (shown as Other)\n", otherCount)
	}

	var xAxis []string

	lineDataMap := map[string][]opts.LineData{}
	for _, status := range allStatuses {
		lineDataMap[status] = []opts.LineData{}
	}

	sortedDays := make([]string, 0, len(dates))
	for day := range dates {
		sortedDays = append(sortedDays, day)
	}
	sort.Strings(sortedDays)

	for _, date := range sortedDays {
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

	c.Printf("    Rendering chart with <white>%d</> data points across <white>%d</> series...\n", len(xAxis), len(allStatuses))

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
			"#365C8D", // Dark Blue
			"#46337E", // Violet
			"#7B414B", // Dark Red, replacing the burnt orange
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
	outFile := outPath + "/daily-issues-open.html"
	file, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	err = graph.Render(file)
	if err != nil {
		return fmt.Errorf("failed to render graph: %w", err)
	}

	c.Printf("    <green>✓</> Wrote %s\n", outFile)

	return nil
}
