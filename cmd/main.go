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
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at,string"`
	Payload   struct {
		Commits []interface {
		} `json:"commits"`
	} `json:"payload"`
}

func getPublicEvents(client *http.Client) ([]Event, error) {
	// construct url from username
	url := fmt.Sprintf("https://api.github.com/users/%s/events", os.Getenv("GITHUB_USERNAME"))
	// create a new http request with the method and url, no body
	req, err := http.NewRequest("GET", url, nil)
	// add the authorization header so that we can access commits to private repos
	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_API_TOKEN")))
	// send the request
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Search query failed: %v", resp.Status)
	}
	var events []Event

	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("Error in decoding json from response body: %s", err)
	}
	return events, nil
	// body, err := ioutil.ReadAll(resp.Body)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	client := &http.Client{
		Timeout: time.Second * 7,
	}
	events, err := getPublicEvents(client)
	if err != nil {
		log.Fatal(err)
	}
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(data))

}
