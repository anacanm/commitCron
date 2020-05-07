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
// if the RepoContents are no longer needed (signaled by the finish channel), then function exits
// (this occurs when the first concurrent request to contributions.GetNumberOfContributionsToday sends a number higher than the upper bound for daily )
func GetRepoContents(url string, result []RepoContent, nRequiredContents int, beenThere map[string]bool, client *http.Client, finish <-chan struct{}, out chan<- error) {
	select {
	// the use of select here is to have a nonblocking receive check for finish, if no finish message has  been set, proceed with the operation
	case <-finish:
		// despite the fact that this is a recursive function, I am able to use close(out) here knowing that it will not get called more than once
		// there will ever only be one message sent to this finish channel, so this statement will execute only one time
		close(out)
		return
	default:
		// update the beenThere array for the current directory
		beenThere[url] = true

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			out <- fmt.Errorf("Error creating http GET request for %v: %v", url, err)
		}

		req.Header.Add("Authorization", fmt.Sprintf("token %v", os.Getenv("GITHUB_API_TOKEN")))

		resp, err := client.Do(req)
		if err != nil {
			out <- fmt.Errorf("Error sending http GET request for %v: %v", url, err)
		}

		// I cannot use "defer resp.Body.Close()" because of this function's recursive-ness and defer's last called, first executed nature: https://tour.golang.org/flowcontrol/13
		// if defer were used, the last called response body would be closed, then the second last called response body, and so on until finally the first called response was closed last.
		// It is better to explicitly repeat myself more than to use defer and allow the resource leek from keeping the connection open.

		//shallowResult is a temporary location to decode the response from the github api into. It is from this shallowResult that we can filter through the data
		var shallowResult []RepoContent
		err = json.NewDecoder(resp.Body).Decode(&shallowResult)
		if err != nil {
			out <- fmt.Errorf("Error decoding json response from %v into []RepoContent: %v", url, err)
			resp.Body.Close()
		}

		for _, value := range shallowResult {
			// if the number of files that are desired to be updated have been reached, initiate exit
			if len(result) == nRequiredContents {
				resp.Body.Close()
				return
			}
			// otherwise, check if the file is a file and if it is allowed to be modified, and append it to the list of files to be modified
			if value.Type == "file" && fileCanBeModified(value.Name) {
				result = append(result, value)
			}
		}

		// if this is reached, then the current directory of the tree has no more files that can be updated, so we must proceed a level deeper
		for _, value := range shallowResult {
			if value.Type == "dir" && !beenThere[value.Links.Self] {
				resp.Body.Close()
				GetRepoContents(value.Links.Self, result, nRequiredContents, beenThere, client, finish, out)
			}
		}

		// if this is reached then the current directory of the tree has no more files that can be updated, nor subdirectories, so now we must return up a level
		// TODO: implement tree structure that will be updated as it is traversed
		resp.Body.Close()
	}

}
