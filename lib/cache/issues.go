package cache

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
)

var IssueColumns = []string{"key", "url", "type", "status", "resolution", "summary", "labels", "creator", "created", "updated", "daysopen"}

func IssueColumnsString() string {
	return strings.Join(IssueColumns, ", ")
}

func IssueColumnsPlaceholders() string {
	return strings.TrimSuffix(strings.Repeat("?, ", len(IssueColumns)), ", ")
}

const CreateIssuesTableSQL = `
	CREATE TABLE "issues" (
	    "key" CHAR(16) NOT NULL, 
	    "url" CHAR(256) NOT NULL,
	    "type" CHAR(32) NOT NULL,
	    "status" CHAR(32) NOT NULL,
	    "resolution" CHAR(32) NOT NULL,
	    "summary" CHAR(256) NOT NULL,
	    "labels" VARCHAR(256) NOT NULL,
	    "creator" CHAR(64) NOT NULL,
	    "created" DATE NOT NULL,
	    "updated" DATE NOT NULL,
	    "daysopen" REAL,
	    PRIMARY KEY (key)
	)
`

type Issue struct {
	Key string
	URL string

	Type   string
	Status string
	// Sprint     string // do we bother with this? it is an internal process thing
	Resolution string

	Summery string
	Labels  []string

	Creator string
	Created time.Time
	Updated time.Time

	// calculated
	DaysOpen sql.NullFloat64
	Closed   time.Time // todo, need to parse events to get this
}

func (i Issue) IsCLosed() bool {
	return i.Status == "Closed"
}

func (cache Cache) UpsertIssueFromJIRA(issue *models.IssueScheme) error {
	stmt, err := cache.DB.Prepare(fmt.Sprintf(`
		INSERT OR REPLACE INTO issues (%s)
		VALUES (%s)
	`, IssueColumnsString(), IssueColumnsPlaceholders()))
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement for issue %s: %w", issue.Key, err)
	}

	// get jira instance url from self field
	parsedURL, err := url.Parse(issue.Self)
	if err != nil {
		return fmt.Errorf("failed to parse issue URL %s: %w", issue.Self, err)
	}

	resolution := ""
	if issue.Fields.Resolution != nil {
		resolution = issue.Fields.Resolution.Name
	}

	createdDate, err := time.Parse("2006-01-02T15:04:05.000-0700", issue.Fields.Created)
	if err != nil {
		return fmt.Errorf("failed to parse Created date %s: %w", issue.Fields.Created, err)
	}
	updatedDate, err := time.Parse("2006-01-02T15:04:05.000-0700", issue.Fields.Updated)
	if err != nil {
		return fmt.Errorf("failed to parse Updated date %s: %w", issue.Fields.Updated, err)
	}
	_, err = stmt.Exec(
		issue.Key,
		fmt.Sprintf("%s://%s/browse/%s", parsedURL.Scheme, parsedURL.Host, issue.Key),
		issue.Fields.IssueType.Name,
		issue.Fields.Status.Name,
		resolution,
		issue.Fields.Summary,
		strings.Join(issue.Fields.Labels, ", "),
		issue.Fields.Creator.DisplayName,
		createdDate,
		updatedDate,
		0, // we calculate this after we get all events
	)

	if err != nil {
		return fmt.Errorf("failed to insert issue %s: %w", issue.Key, err)
	}
	stmt.Close()

	return nil
}

func (cache Cache) QueryForIssues(qfmt string, a ...any) (*[]Issue, error) {
	q := fmt.Sprintf(qfmt, a...)

	rows, err := cache.DB.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare issue query '%s': %w", q, err)
	}
	defer rows.Close()

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to run issue query: %w", err)
	}

	issues := make([]Issue, 0)

	var labels string
	for rows.Next() {
		issue := Issue{}
		err = rows.Scan(
			&issue.Key,
			&issue.URL,
			&issue.Type,
			&issue.Status,
			&issue.Resolution,
			&issue.Summery,
			&labels,
			&issue.Creator,
			&issue.Created,
			&issue.Updated,
			&issue.DaysOpen,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pr: %w", err)
		}

		issue.Labels = strings.Split(labels, ", ")

		issues = append(issues, issue)
	}

	return &issues, nil
}

func (cache Cache) GetIssue(key string) (*Issue, error) {
	issues, err := cache.QueryForIssues(`
	SELECT %s
	FROM issues
	WHERE
	    key = '%s'
	`, IssueColumnsString(), key)

	if err != nil {
		return nil, fmt.Errorf("failed to query for issue %s: %w", key, err)
	}

	if len(*issues) != 1 {
		return nil, nil
	}

	issue := (*issues)[0]
	return &issue, nil
}

func (cache Cache) GetAllIssues() (*[]Issue, error) {
	return cache.QueryForIssues(`
		SELECT %s FROM issues
	`, IssueColumnsString())
}

func (cache Cache) GetIssuesCreatedInDateRange(from, to time.Time) (*[]Issue, error) {
	return cache.QueryForIssues(`
		SELECT %s FROM issues
		WHERE
		    created BETWEEN '%s' AND '%s'
			%s
	`, IssueColumnsString(), from.Format("2006-01-02"), to.Format("2006-01-02"))
}

/*
func (cache Cache) GetRepoIssuesOpenForDateRange(repos []string, from, to time.Time) (*[]Issue, error) {
	repoClause := ""
	if len(repos) > 0 {
		repoClause = " AND repo in ('" + strings.Join(repos, "', '") + "')"
	}

	return cache.QueryForIssues(fmt.Sprintf(`
		SELECT %[1]s  FROM issues
		WHERE
		    (created BETWEEN '%[2]s' AND '%[3]s' OR
		    closed BETWEEN '%[2]s' AND '%[3]s' OR
		    closed < '1977-7-7')
		    %[4]s
	`, ColumnsIssues, from.Format("2006-01-02"), to.Format("2006-01-02"), repoClause))
}


func (cache Cache) UpsertIssueStats(repo string, number int, daysOpen, daysWaiting, daysToFirst float64) error {
	stmt, err := cache.DB.Prepare(`
		UPDATE issues
		SET daysopen = ?,
		    dayswaiting = ?,
		    daystofirst = ?
		WHERE
		    repo=? AND
			number=?;
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert stats statement for pr %d: %w", number, err)
	}

	if _, err = stmt.Exec(daysOpen, daysWaiting, daysToFirst, repo, number); err != nil {
		return fmt.Errorf("failed to insert stats statement for pr %s#%d: %w", repo, number, err)
	}
	stmt.Close()

	return nil
}

*/
