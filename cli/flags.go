package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type FlagData struct {
	Url       string
	User      string
	Token     string
	JQL       string
	Fields    []string
	Expand    []string
	CachePath string
	// FullFetch bool // not important for jira?
}

func configureFlags(root *cobra.Command) error {
	flags := FlagData{}
	pflags := root.PersistentFlags()

	pflags.StringVarP(&flags.Url, "url", "", "", "jira instance url")
	pflags.StringVarP(&flags.User, "user", "u", "", "jira user")
	pflags.StringVarP(&flags.Token, "token", "t", "", "jira oauth token (JIRA_TOKEN)")
	pflags.StringVarP(&flags.JQL, "jql", "q", "", "jira jql query to list all issues")
	pflags.StringSliceVarP(&flags.Fields, "fields", "f", nil, "jira fields to fetch seperated by commas")
	pflags.StringSliceVarP(&flags.Expand, "expand", "e", nil, "jira fields to expand seperated by commas")
	pflags.StringVarP(&flags.CachePath, "cache", "c", "", "path to sqllite3 db to use as cache")

	// binding map for viper/pflag -> env
	m := map[string]string{
		"url":    "JIRA_URL",
		"user":   "JIRA_USER",
		"jql":    "JIRA_JQL",
		"token":  "JIRA_TOKEN",
		"fields": "JIRA_FIELDS",
		"expand": "JIRA_EXPAND",
		"cache":  "CACHE_DB_FILE",
	}

	for name, env := range m {
		if err := viper.BindPFlag(name, pflags.Lookup(name)); err != nil {
			return fmt.Errorf("error binding '%s' flag: %w", name, err)
		}

		if env != "" {
			if err := viper.BindEnv(name, env); err != nil {
				return fmt.Errorf("error binding '%s' to env '%s' : %w", name, env, err)
			}
		}
	}

	return nil
}

func GetFlags() FlagData {
	// for some reason we don't get a proper array back from viper for authors so fix it liek this for now TODO FIX
	fields := viper.GetStringSlice("fields")
	if len(fields) != 0 {
		fields = strings.Split(fields[0], ",")
	}

	expand := viper.GetStringSlice("expand")
	if len(expand) != 0 {
		expand = strings.Split(expand[0], ",")
	}

	// there has to be an easier way....
	return FlagData{
		Url:       viper.GetString("url"),
		User:      viper.GetString("user"),
		Token:     viper.GetString("token"),
		JQL:       viper.GetString("jql"),
		Fields:    fields,
		Expand:    expand,
		CachePath: viper.GetString("cache"),
	}
}
