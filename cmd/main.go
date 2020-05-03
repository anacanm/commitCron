package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Event is the struct that will hold the unmarshalled data returned from the github events api
// from the api, all that is needed is the types and the commits
// I will iterate over the array of events that occured today, and for events that are type "PushEvent", I will count the # of commits
type Event struct {
	CreatedAt time.Time `json:"created_at,string"`
	Type      string    `json:"type"`
	Payload   struct {
		RefType string `json:"ref_type"`
		Commits []interface {
		} `json:"commits"`
	} `json:"payload"`
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

func getNumberOfContributionsToday(client *http.Client) (int, error) {
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

	// repoExists is a map of string repo names to bool values
	// this allows me to reduce calls to the github api to check if a repo exists, I may have already stored it
	// repoExists := make(map[string]bool)

	numberOfContributionsToday := 0
	for _, event := range events {
		if sameDay(event.CreatedAt) {
			// if the event was created today then check if there were any contributions made today
			if event.Type == "CreateEvent" {
				// if a repository is created, it counts as a contribution (creating a branch does not)
				if event.Payload.RefType == "repository" {

					numberOfContributionsToday++
				}
				// * NOTE: a possible source of miscalculation lies in the fact that through the events API,
				// * I have access to Create repository events, yet not whether the repositories ex
			} else if event.Type == "PullRequestEvent" {
				numberOfContributionsToday++
			} else if event.Type == "PushEvent" {

				if sameDay(event.CreatedAt) {
					numberOfContributionsToday += len(event.Payload.Commits)
				} else {
					fmt.Printf("\n\nnot same day?: %v", event.CreatedAt.Local())
					break
				}
			}
		} else {
			// since github returns the events sorted in reverse chronological order (most recent first, oldest last),
			// if an event that did not occur today is reached, I need not traverse any more events, they all occured before today
			break
		}
	}

	return numberOfContributionsToday, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	client := &http.Client{
		Timeout: time.Second * 7,
	}
	numberOfContributions, err := getNumberOfContributionsToday(client)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n\nNumber of contributions: %v", numberOfContributions)
}
