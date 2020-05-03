// Package contributions provides convenient access to the number of contributions the authenticated user has made today
// requires 2 environment variables to work: GITHUB_USERNAME=your github username, and GITHUB_API_TOKEN=a github personal access api token that you create
// create the token here: https://github.com/settings/tokens and make sure to give it full access to the "repo" scope. This is needed so that contributions to
// private repositories are counted
package contributions

import (
	"encoding/json"
	"fmt"
	"os"

	"net/http"
	"time"
)

// Event is used to hold the relevant unmarshalled data returned from the github events api
type Event struct {
	CreatedAt time.Time `json:"created_at,string"`
	Type      string    `json:"type"`
	Payload   struct {
		Ref     string `json:"ref"`
		RefType string `json:"ref_type"`
		Commits []struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
		} `json:"commits"`
	} `json:"payload"`
	Repo struct {
		Name string `json:"name"`
	} `json:"repo"`
}

// message is a struct to Unmarshal the json response into when accessing the github repos api
type message struct {
	Message string `json:"message"`
}

// sameDay returns true if the other Time (in this case, the git push time), occured on the same day as it currently is
func sameDay(other time.Time) bool {
	// convert both times to local, since the github profile page reflects commits according to your local time
	thisYear, thisMonth, thisDay := time.Now().Date()
	otherYear, otherMonth, otherDay := other.Local().Date()
	if thisYear != otherYear {
		return false
	}
	if thisMonth != otherMonth {
		return false
	}
	if thisDay != otherDay {
		return false
	}
	//at this point, we know the Time at occurence is the same day as this was pushed

	return true
}

func repoExists(repoName string, repoMap map[string]bool, client *http.Client) (bool, error) {
	value, present := repoMap[repoName]
	// first, I check to see if I've already queried the github api for this repo
	if present {
		// if I've already queried the github api, then I can simply return what I already know
		return value, nil
	}
	// otherwise, I need to query the github api
	url := fmt.Sprintf("https://api.github.com/repos/%v", repoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("Error creating request to accesses %v: %v", url, err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_API_TOKEN")))

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("Error in querying %v: %v", url, err)
	}
	defer resp.Body.Close()

	var mess message
	err = json.NewDecoder(resp.Body).Decode(&mess)
	if err != nil {
		return false, fmt.Errorf("Error in decoding the json response from querying %v: %v", url, err)
	}
	if mess.Message == "" {
		// no message field indicates that the repo exists
		// update the map and return true, no errors
		repoMap[repoName] = true
		return true, nil
	}
	// all messages other than "Not Found" indicate that the repo exists, eg. "Moved Permanently"
	if mess.Message == "Not Found" {
		// update the map and return false, no errors
		repoMap[repoName] = false
		return false, nil
	}
	return true, nil
}

// GetNumberOfContributionsToday returns the number of contributions made for the authorized user
// takes an http.Client as a parameter, encouraging the user to create and specify their own client
// for information how to do so: https://golang.org/pkg/net/http/
// requires GITHUB_USERNAME and GITHUB_API_TOKEN to be set environment variables
// GITHUB_API_TOKENs can be created here: https://github.com/settings/tokens, this api token needs full access to the repo scope
func GetNumberOfContributionsToday(client *http.Client) (int, error) {
	// construct url from username
	url := fmt.Sprintf("https://api.github.com/users/%s/events", os.Getenv("GITHUB_USERNAME"))
	// create a new http request with the method and url, no body
	req, err := http.NewRequest("GET", url, nil)
	// add the authorization header so that we can access commits to private repos
	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_API_TOKEN")))
	// send the request
	resp, err := client.Do(req)

	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	// checks the status code
	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("Search query failed: %v", resp.Status)
	}
	var events []Event

	// Unmarshals the data into the an array of Events
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return -1, fmt.Errorf("Error in decoding json from response body: %s", err)
	}

	// repoMap is a map of string repo names to bool values
	// this allows me to reduce calls to the github api to check if a repo exists, I may have already stored it
	repoMap := make(map[string]bool)

	// things that I have found count as contributions to GitHub:
	// 	commits each one that is merged counts as a contribution, including the merge request itself. Pushing to branches does not count as a contribution
	// 	creating a master branch (which does not show up as a commit in events)
	// 	creating a repository
	// 	pull requests

	numberOfContributionsToday := 0
	for _, event := range events {
		if sameDay(event.CreatedAt) {
			repositoryExists, err := repoExists(event.Repo.Name, repoMap, client)
			if err != nil {
				return -1, err
			}
			if repositoryExists {
				// if the event was created today, and the repository exists, then check if there were any contributions made today
				if event.Type == "CreateEvent" {
					// if a repository was created and still exists, it counts as a contribution
					// also, creating a master branch counts as a contribution, creating other branches do not
					if event.Payload.RefType == "repository" || event.Payload.Ref == "master" {
						numberOfContributionsToday++
					}
				} else if event.Type == "PullRequestEvent" {
					numberOfContributionsToday++
				} else if event.Type == "PushEvent" {
					for _, commit := range event.Payload.Commits {
						if commit.Message != "Update README.md" {
							numberOfContributionsToday++
						}
					}
				}

			}
		} else {
			break
		}
	}

	return numberOfContributionsToday, nil
}
