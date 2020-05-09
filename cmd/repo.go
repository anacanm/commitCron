package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// RepoContent holds the necessary information about the contents of a repository
type RepoContent struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	SHA   string `json:"sha"`
	Type  string `json:"type"`
	Links struct {
		Self string `json:"self"`
	} `json:"_links"`
	Error error `json:",omitempty"`
}

// fileCanBeModified is a helper method that helps determine whether or not the file can have a comment safely inserted
// this is to help ensure that important files such as go.mod are not modified, (even though you should not have this code running in a repository with important code)
// currently I've only added support for languages that support // comments
func fileCanBeModified(fileName string) bool {
	acceptedSuffixes := [6]string{".js", ".java", ".go", ".c", ".cpp", ".txt"}
	for _, suffix := range acceptedSuffixes {
		if strings.HasSuffix(fileName, suffix) {
			return true
		}
	}
	return false

}

// GetRepoContents sends (on the out channel) the first n RepoContents in a repository that are able to be modified (ie. not dirs or important files)
// TODO: update documentation (mainly the func doc)
// if the RepoContents are no longer needed (signaled by the terminate channel), then function exits
// (this occurs when the first concurrent request to contributions.GetNumberOfContributionsToday sends a number higher than the upper bound for daily )
func GetRepoContents(url string, result []RepoContent, nRequiredContents int, client *http.Client, output chan []RepoContent, terminate chan struct{}, errorChan chan<- error) {
	select {
	// the use of select here is to have a nonblocking receive check for terminate, if no terminate message has  been set, proceed with the operation
	case <-terminate:
		// this case needs to initiate the recursive exit of the function
		// it does so by sending an additional message to the terminate channel before it returns, which will be consumed by the function below it on the call stack, and a new one will be sent.
		// this means that the last message sent (before the root level function is returned from), will not be consumed
		// this helps reduce the resources consumed, and will safely exit the function so that the goroutine calling it may exit

		// ! NOTE: it may be better to create a select { case <-terminate } in the loop that recursively calls this function so that no new HTTP requests are made and to keep the growth of this goroutine to a minimum
		terminate <- struct{}{}
		// TODO: below
		// * need to close the out channel outisde of GetRepoContents
		// * need to drain and close the terminate channel outside of GetRepoContents, (there will be one message remaining after the last return statement)
		return

	default:
		if len(result) == nRequiredContents {
			// * proper exit case: send the result to the output channel, initiate termination
			// the output channel should not block on send (have a buffer where length < capacity), so that this function will be able to send the result and immediately begin termination quietly on its own.

			// If the output channel is blocking (which is not desired), then the goroutine that this is spawned in will have to wait until someone receives on the output channel to begin termination
			output <- result
			terminate <- struct{}{}
			return
		}

		// create new HTTP request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			errorChan <- fmt.Errorf("Error creating http GET request for %v: %v", url, err)
			terminate <- struct{}{}
			return
		}

		// add Authorization header with user's github api token
		// for info on creating an api token: https://github.com/settings/tokens
		// for this project, the api token needs access to the full repo scope
		req.Header.Add("Authorization", fmt.Sprintf("token %v", os.Getenv("GITHUB_API_TOKEN")))

		// send request
		resp, err := client.Do(req)
		if err != nil {
			errorChan <- fmt.Errorf("Error sending http GET request for %v: %v", url, err)
			terminate <- struct{}{}
			return
		}

		// * NOTE: I cannot use "defer resp.Body.Close()" because of this function's recursive-ness and defer's last called, first executed nature: https://tour.golang.org/flowcontrol/13
		// * if defer were used, the last called response body would be closed, then the second last called response body, and so on until finally the first called response was closed last.
		// * It is better to explicitly repeat myself more than to use defer and allow the resource leek from keeping the connection open.

		//shallowResult is a temporary location to decode the response from the github api request into. It is from this shallowResult that we can filter through the data
		var shallowResult []RepoContent
		err = json.NewDecoder(resp.Body).Decode(&shallowResult)
		if err != nil {
			errorChan <- fmt.Errorf("Error decoding json response from %v into []RepoContent: %v", url, err)
			terminate <- struct{}{}
			resp.Body.Close()
			return
		}

		// although iterating over shallowResult two separate times has a complexity of 0(2n), I believe that due to the nature of directories being small in breadth
		// n should never get to be large enough such that the complexity would result in a negative impact on performance
		// I weigh the clarity of the two separate iterations to be more important than the possible minimal performance benefit from a more efficient traversal

		for _, value := range shallowResult {
			// if the number of files that are desired to be updated have been reached, initiate exit
			if len(result) == nRequiredContents {
				output <- result
				terminate <- struct{}{}
				resp.Body.Close()
				return
			}
			// otherwise, check if the value is a file and if it is allowed to be modified, and append it to the list of files to be modified
			if value.Type == "file" && fileCanBeModified(value.Name) {
				result = append(result, value)
				// result only ever grows by 1 at a time, so I can safely check len(result) == nRequiredContents without having to worry about passing nRequiredContents
			}
		}

		// if this is reached, then the current directory of the tree has no more files that can be updated, so we must proceed a level deeper
		// we do so by recursing to a new subdirectory in the repository, which requires a new HTTP request to the api specifying that we want the new subdirectory
		for _, value := range shallowResult {
			select {
			// to prevent unecessary recursive calls being made, check to see if a terminate message has been received
			// this select case offers a slight improvement: although the general select case that wraps this entire function ensures that no extra operations are performed in that function call if a terminate message is received,
			// it does not prevent unnecessary calls that will be immediately rejected, which is what this select case does
			// this select case becomes helpful when a terminate message has been sent between the timeframe of the function call beginning and when this loop is finished: if that were the case,
			// then without this select case, each subdirectory iterated over (assuming that len(result) < nRequiredContents) would result in a new recursive call to GetRepoContents,
			// which would immediately be exited early from due to the general select case statement wrapping the function.
			// Therefore, this select statement prevents the pointless growing of the call stack, and allows the function to initiate termination immediately
			case <-terminate:
				// if the terminate message has been received, initiate termination by sending a terminate message to the call below this on the call stack and exit this recusive call
				terminate <- struct{}{}
				return

			default:
				// if the number of files that are desired to be updated have been reached, initiate exit
				if len(result) == nRequiredContents {
					output <- result
					terminate <- struct{}{}
					resp.Body.Close()
					return
				}
				if value.Type == "dir" {
					GetRepoContents(value.Links.Self, result, nRequiredContents, client, output, terminate, errorChan)
				}
			}
		}

		// if this is reached then the current directory of the tree has no more files that can be updated, nor subdirectories to go into, so now we return up a level in the directory structure (down on the call stack)

		// if the function is about to return from the root directory, then send whatever content has been accumulated
		// this is to ensure that even if the number of modifiable files is less than nRequiredContents, that the modifiable content (if any) is sent
		// however, if content has already been sent, then this will be a duplicate send, but this is okay since the channel will only be read from once
		// the output channel should have a buffer of 2 so that in the case of a second send, the function does not block
		if url == fmt.Sprintf("https://api.github.com/repos/%v/%v/contents", os.Getenv("GITHUB_USERNAME"), os.Getenv("REPO_NAME")) {
			output <- result
		}
		resp.Body.Close()
		return
	}

}
