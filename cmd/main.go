package main

import (
	"fmt"
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

	// contributionChannel is an unbuffered channel that will receive the numberOfContributions
	contributionChannel := make(chan contributions.ContributionItem)

	// TODO: make numberOfContributionsToMake either user-inputted or a random number within a range to provide the illusion of human commits
	numberOfContributionsToMake := 5

	go contributions.GetNumberOfContributionsToday(client, contributionChannel)

	// "Don't communicate by sharing memory, share memory by communicating": https://www.youtube.com/watch?v=PAAkCSZUG1c&t=2m48s

	// * NOTE: Initialize result with a capacity of numberOfContributionsToMake so that no additional allocation will be needed

	// TODO: if a message from main is sent to finishGetRepo, finishGetRepo needs to be drained and closed
	repoContentsURL := fmt.Sprintf("https://api.github.com/repos/%v/%v/contents", os.Getenv("GITHUB_USERNAME"), os.Getenv("REPO_NAME"))

	// ! all of the channels used by GetRepoContents should be buffered so that the function can send the necessary message (whether it be an error or result) and immediately begin termination
	getRepoOutput := make(chan []RepoContent, 2)
	terminateGetRepo := make(chan struct{}, 1)
	getRepoContentsErrorChan := make(chan error, 1)

	go func() {
		// GetRepoContents is wrapped in this anonymous function because it is recursive and therefore calling defer close(channelName) would not work well.
		// Therefore, it is best to simply wrap it in a small anonymous function that gives the flexibility desired

		// NOTE: cannot call defer close(getRepoOutput) or defer close(getRepoContentsErrorChan) until after the below select statement because a closed channel never blocks
		// this means that in the below select case, if the function were to have succeeded sending the data AND terminating before the select statement was reached, the error channel would be closed
		// , and therefore readable from (reading it will return a nil error when one was never sent), so it would be selected when no error was sent.

		defer close(terminateGetRepo)

		GetRepoContents(repoContentsURL, make([]RepoContent, 0, numberOfContributionsToMake), numberOfContributionsToMake, client, getRepoOutput, terminateGetRepo, getRepoContentsErrorChan)
	}()

	contributionResult := <-contributionChannel
	if contributionResult.Err != nil {
		log.Fatalf("Error getting contributions: %v", contributionResult.Err)
	}

	// 4 is an arbitrary number that I chose so that if I'd already made plenty of commits on a day, that I wouldn't overdo my commits
	if contributionResult.NumberContributions < 100 {
		// if we want to make contributions, we need to gracefully handle possible errors, and then procede
		select {
		case err := <-getRepoContentsErrorChan:
			// close the channels,
			close(getRepoContentsErrorChan)
			close(getRepoOutput)
			log.Fatalf("Error getting repo contents from %v: %v", repoContentsURL, err)

		case contents := <-getRepoOutput:
			close(getRepoContentsErrorChan)
			close(getRepoOutput)

			updateErrorChan := make(chan error, cap(contents))
			updateDonechan := make(chan struct{}, cap(contents))
			UpdateFilesAndCreateRemaining(contents, client, updateErrorChan, updateDonechan)

			for numMessagesReceived := 0; numMessagesReceived < cap(contents); numMessagesReceived++ {
				select {
				case err := <-updateErrorChan:
					fmt.Println(err)
				case <-updateDonechan:
					// do nothing, this is just to drain the responses
				}
			}
		}
		// repoName is the repository that you want to access
		// path to file is the relative (relative to the repo) path that
	} else {
		// if we do not in fact want to make any contributions, since we have achieved our daily quota, then we should instruct the function to terminate
		// there are three distinct states that GetRepoContents can be in:
		// 	1. it has found an error, communicated it over the channel, and begun termination on its own. In this case, we should do nothing more then drain the error
		//	2. it has already completed getting the desired content from the repo, communicated it over the channel, and begun termination on its own. Again, do nothing other than drain
		//	3. neither an error or completion has occured: we should instruct the function to terminate gracefully as it is no longer needed
		select {
		case <-getRepoContentsErrorChan:
			// do nothing, it is cleaning itself up
		case <-getRepoOutput:
			// do nothing, it is cleaning itself up
		default:
			// instruct the function to terminate
			terminateGetRepo <- struct{}{}
		}
	}
}
