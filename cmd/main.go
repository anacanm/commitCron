package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/anacanm/contributionCron/contributions"
	"github.com/joho/godotenv"
)

func main() {
	// first I need to ensure that I have access to the env variables
	_, present := os.LookupEnv("ENV")
	// if the environment variables are not accessible automatically, ie. running in development with a .env file, then load them from the .env file
	if !present {
		err := godotenv.Load()
		if err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}
	}

	// create an http Client with a 7 second timeout to be used by all goroutines:
	// From https://golang.org/src/net/http/client.go: 
	// "Clients should be reused instead of created as needed. Clients are safe for concurrent use by multiple goroutines."
	client := &http.Client{
		Timeout: time.Second * 7,
	}

	// nContributions is an unbuffered channel that will receive the numberOfContributions
	contributionChannel := make(chan contributions.ContributionItem) 


	go contributions.GetNumberOfContributionsToday(client, contributionChannel)


	contributionResult := <-contributionChannel
	if contributionResult.Err != nil {
		log.Fatalf("Error getting contributions: %v", contributionResult.Err)
	}


	// if the user has committed less than 4 times, make somewhere between 4 and 8 (random, inclusive) commits to give the illusion of normal commits
	// 4 is an arbitrary number that I chose so that if I'd already made plenty of commits on a day, that I wouldn't overdo my commits
	if contributionResult.NumberContributions < 4 {
		// repoName is the repository that you want to access
		// path to file is the relative (relative to the repo) path that
		if err := UpdateFile(os.Getenv("REPO_NAME"), os.Getenv("PATH_TO_FILE"), client); err != nil {
			log.Fatalf("Error updating %v in %v: %v", os.Getenv("PATH_TO_FILE"), os.Getenv("REPO_NAME"), err)
		}
	}
}
