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
		Ref     string `json:"ref"`
		RefType string `json:"ref_type"`
		Commits []interface {
		} `json:"commits"`
	} `json:"payload"`
	Repo struct {
		Name string `json:"name"`
	} `json:"repo"`
}

// Message is a struct to Unmarshal the json response into when accessing the github repos api
type Message struct {
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

	var message Message
	err = json.NewDecoder(resp.Body).Decode(&message)
	if err != nil {
		return false, fmt.Errorf("Error in decoding the json response from querying %v: %v", url, err)
	}
	if message.Message == "" {
		// no message field indicates that the repo exists
		// update the map and return true, no errors
		repoMap[repoName] = true
		return true, nil
	}
	// all messages other than "Not Found" indicate that the repo exists, eg. "Moved Permanently"
	if message.Message == "Not Found" {
		// update the map and return false, no errors
		repoMap[repoName] = false
		return false, nil
	}
	return true, nil
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
					numberOfContributionsToday += len(event.Payload.Commits)
				}

			}
		} else {
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
