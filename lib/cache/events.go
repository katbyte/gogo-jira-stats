package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
)

var EventColumns = []string{"key", "author", "date", "field", "[from]", "[to]"}

func EventColumnsString() string {
	return strings.Join(EventColumns, ", ")
}

func EventColumnsPlaceholders() string {
	return strings.TrimSuffix(strings.Repeat("?, ", len(EventColumns)), ", ")
}

const CreateEventsTableSQL = `
	CREATE TABLE "events" (
	    "key" CHAR(16) NOT NULL,
	    "author" CHAR(64) NOT NULL,
	    "date" DATE NOT NULL,
	    "field" CHAR(32) NOT NULL,
	    "from" CHAR(32) NOT NULL,
	    "to" CHAR(32) NOT NULL,
	    PRIMARY KEY (key, date, field)
	)
`

type Event struct {
	Key    string
	Author string
	Date   time.Time
	Field  string
	From   string
	To     string
}

func (cache Cache) UpsertEventsFromIssue(issue *models.IssueScheme) (*int, error) {

	count := 0
	if issue.Changelog == nil {
		return &count, nil
	}

	for _, change := range issue.Changelog.Histories {

		author := ""
		if change.Author != nil {
			author = change.Author.DisplayName
		}
		date := change.Created

		for _, item := range change.Items {
			stmt, err := cache.DB.Prepare(fmt.Sprintf(`
				INSERT OR REPLACE INTO events (%s)
				VALUES (%s)
			`, EventColumnsString(), EventColumnsPlaceholders()))
			if err != nil {
				return nil, fmt.Errorf("failed to prepare insert statement for issue %s changelog: %w", issue.Key, err)
			}

			_, err = stmt.Exec(
				issue.Key,
				author,
				date,
				item.Field,
				item.FromString,
				item.ToString,
			)

			if err != nil {
				return nil, fmt.Errorf("failed to insert issue %s changelog: %w", issue.Key, err)
			}
			stmt.Close()

			count++
		}
	}

	return &count, nil
}

func (cache Cache) GetIssueEventsForField(key, field string) ([]Event, error) {
	rows, err := cache.DB.Query(fmt.Sprintf(`
		SELECT %s
		FROM events
		WHERE
			key='%s' AND
		    field='%s' ORDER BY date
	`, EventColumnsString(), key, field))
	if err != nil {
		return nil, fmt.Errorf("failed to query events for issue %s for field %s: %w", key, field, err)
	}
	defer rows.Close()

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to get events for issue %s for field %s: %w", key, field, err)
	}

	var events []Event
	for rows.Next() {
		e := Event{}
		err = rows.Scan(
			&e.Key,
			&e.Author,
			&e.Date,
			&e.Field,
			&e.From,
			&e.To,
		)

		events = append(events, e)
		if err != nil {
			return nil, fmt.Errorf("failed to scan events for issue %s for field %s: %w", key, field, err)
		}
	}

	return events, nil
}
