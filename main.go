package main

import (
	"os"

	c "github.com/gookit/color" // nolint: misspell
	"github.com/katbyte/gogo-jira-stats/cli"
	"github.com/katbyte/gogo-jira-stats/lib/clog"
)

const cmdName = "gogo-jira-stats"

func main() {
	cmd, err := cli.Make(cmdName)
	if err != nil {
		clog.Log.Errorf(c.Sprintf("<red>%s: building cmd</> %v", cmdName, err))

		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		clog.Log.Errorf(c.Sprintf("<red>%s:</> %v", cmdName, err))

		os.Exit(1)
	}

	os.Exit(0)
}
