package cli

import (
	"fmt"
	"os"
	"time"

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

	// populate dates
	dates := map[string]DailyOpenIssues{}
	for day := from; day.Before(to); day = day.AddDate(0, 0, 1) {
		k := day.Format("2006-01-02")
		dates[k] = DailyOpenIssues{
			Date: day,
		}
	}

	// get all open prs for range (this should be opened no created but currently we don't have closed date in the cache or easily accessible
	/*prs, err := theCache.GetIssuesCreatedInDateRange(from, to)
	if err != nil {
		return fmt.Errorf("getting PRs: %w", err)
	}*/

	// for now lets just go over ALL issues until we can query for any open within a date
	issues, err := theCache.GetAllIssues()
	if err != nil {
		return fmt.Errorf("getting issuess: %w", err)

	}
	// for each pr in range
	for _, i := range *issues {
		opened := time.Date(i.Created.Year(), i.Created.Month(), i.Created.Day(), 0, 0, 0, 0, time.UTC)

		closed := to
		/*closed := pr.Closed
		if pr.State == "open" {
			closed = time.Now()
		}*/
		closed = time.Date(closed.Year(), closed.Month(), closed.Day(), 0, 0, 0, 0, time.UTC)

		// figure out timeline of events that matter
		events, err := theCache.GetIssueEventsForField(i.Key, "status")
		if err != nil {
			return fmt.Errorf("getting events for %s: %w", i.Key, err)
		}

		// for each day from open to closed (or now) count this PR using the above array to figure out its "state"
		// by playing back events to "set the state" until the events
		status := "Unknown"

		eventIndex := 0
		for day := opened; ; day = day.AddDate(0, 0, 1) {

			// skip data before the "from" date
			if day.Before(from.AddDate(0, 0, -1)) {
				continue
			}

			// skip ones with no events
			if len(events) == 0 {
				break
			}

			// data key
			k := day.Format("2006-01-02")

			dayData := dates[k]
			dayData.Total++

			// go through all events for "today" and set the status
			for ; eventIndex < len(events) && events[eventIndex].Date.Before(day.AddDate(0, 0, 1)); eventIndex++ {
				status = events[eventIndex].To
			}

			dayData.Statuses[status] = dayData.Statuses[status] + 1
			dates[k] = dayData

			// check is here so PRs open for less than 1 day are counted
			if !day.Before(closed) {
				break
			}

			if day.After(to) {
				break
			}
		}
	}
	/*
		var xAxis []string
		var lineOpen, lineBlocked, lineWaiting, lineWaitingOver, lineTrendLine []opts.LineData

		data := [][]string{{"date", "total", "open", "blocked", "waiting", "waiting-over", "approved", "7 day trend"}}

		days := make([]string, 0, len(dates))
		for day := range dates {
			days = append(days, day)
		}
		sort.Strings(days)

		for _, date := range days {
			day := dates[date]
			data = append(data, []string{date, strconv.Itoa(day.Total), strconv.Itoa(day.Open), strconv.Itoa(day.Blocked), strconv.Itoa(day.Waiting), strconv.Itoa(day.WaitingOver), strconv.Itoa(day.Approved)})

			if day.Date.Before(from.AddDate(0, 0, -1)) {
				continue
			}

			if day.Date.After(to) {
				continue
			}

			xAxis = append(xAxis, date)
			// totalLine = append(totalLine, opts.LineData{Value: day.Total})
			lineOpen = append(lineOpen, opts.LineData{Value: day.Open})
			lineBlocked = append(lineBlocked, opts.LineData{Value: day.Blocked})
			lineWaiting = append(lineWaiting, opts.LineData{Value: day.Waiting})
			lineWaitingOver = append(lineWaitingOver, opts.LineData{Value: day.WaitingOver})
			lineTrendLine = append(lineTrendLine, opts.LineData{Value: day.TrendSevenDay})
		}

		// write raw data
		file, err := os.Create(outPath + "/daily-prs-open.csv")
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		csv := csv.NewWriter(file)
		defer csv.Flush()

		for _, r := range data {
			err := csv.Write(r)
			if err != nil {
				panic(err)
			}
		}

		var repoShortNames []string
		for _, r := range repos {
			repoShortNames = append(repoShortNames, gh.RepoShortName(r))
		}

		// render graph
		graph := charts.NewLine()
		graph.SetGlobalOptions(
			// charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros}),
			charts.WithTitleOpts(opts.Title{
				Title:    strings.Join(repoShortNames, ",") + " PRs Open (daily)",
				Subtitle: "By State: open, waiting, waiting (over 14 days), blocked, approved",
				Left:     "center", // nolint:misspell
			}),

			charts.WithXAxisOpts(opts.XAxis{
				Name: "Date",
				// AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} x-unit"},
			}),
			charts.WithYAxisOpts(opts.YAxis{
				Name: "PRs",
				// AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} x-unit"},
			}),
			charts.WithInitializationOpts(opts.Initialization{
				Width:  "1500px",
				Height: "750px",
			}),
			charts.WithColorsOpts(opts.Colors{"#C13530", "#2E4555", "#62A0A8", "#5470c6", "#000000"}),
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

		graph.AddSeries("Blocked", lineBlocked)
		graph.AddSeries("Waiting Over 14", lineWaitingOver)
		graph.AddSeries("Waiting", lineWaiting)
		graph.AddSeries("Open", lineOpen).SetSeriesOptions(prStackOps...)

		graph.AddSeries("Total - 7 day avg", lineTrendLine)
		// Where the magic happens
		file, err = os.Create(outPath + "/daily-prs-open.html")
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		err = graph.Render(file)
		if err != nil {
			return fmt.Errorf("failed to render graph graph: %w", err)
		}
	*/
	return nil
}
