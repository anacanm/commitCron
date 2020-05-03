package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func getPublicEvents(client *http.Client) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/events", os.Getenv("GITHUB_USERNAME"))
	req, err := http.NewRequest("GET", url, nil)

	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_API_TOKEN")))
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Search query failed: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	client := &http.Client{
		Timeout: time.Second * 7,
	}
	response, err := getPublicEvents(client)
	if err != nil {
		log.Fatal(err)
	}
	// the burner.json file is being used in development to look through the json response of the github api
	if err := ioutil.WriteFile("burner.json", response, 0644); err != nil {
		log.Fatal(err)
	}

}
