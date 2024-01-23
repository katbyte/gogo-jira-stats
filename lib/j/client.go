package j

import (
	"context"

	jira "github.com/ctreminiom/go-atlassian/jira/v3"
)

type Instance struct {
	URL   string
	User  string
	Token string
}

func NewInstance(url, user, token string) Instance {
	return Instance{
		URL:   url,
		User:  user,
		Token: token,
	}
}

func (i Instance) NewClient() (*jira.Client, context.Context, error) {
	ctx := context.Background()

	client, err := jira.New(nil, i.URL)
	if err != nil {
		return nil, nil, err
	}

	client.Auth.SetBasicAuth(i.User, i.Token)

	return client, ctx, nil
}

/*
func (t Token) NewClient() (*github.Client, context.Context) {
	ctx := context.Background()

	// use retryablehttp to handle rate limiting
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 7
	retryClient.Logger = clog.Log

	// github is.. special using 403 instead of 429 for rate limiting so we need to handle that here :(
	retryClient.Backoff = func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
		if resp != nil && resp.StatusCode == 403 {
			// get x-rate-limit-reset header
			reset := resp.Header.Get("x-ratelimit-reset")
			if reset != "" {
				i, err := strconv.ParseInt(reset, 10, 64)
				if err == nil {
					utime := time.Unix(i, 0)
					wait := utime.Sub(time.Now()) + time.Minute // add an extra min to be safe
					clog.Log.Errorf("ratelimited, parsed x-ratelimit-reset, waiting for %s", wait.String())
					return wait
				}
				clog.Log.Errorf("unable to parse x-ratelimit-reset header: %s", err)
			}
		}

		return retryablehttp.DefaultBackoff(min, max, attemptNum, resp)
	}
	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if resp.StatusCode == 403 {
			return true, nil
		}

		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}

	if t := t.Token; t != nil {
		t := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *t},
		)
		retryClient.HTTPClient = oauth2.NewClient(ctx, t)
	}

	return github.NewClient(retryClient.StandardClient()), ctx
}
*/
