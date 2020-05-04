package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
)

// FileResponse holds the necessary data from the response for GETting a file
type FileResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
	Message  string `json:"message"`
}

// CreateResponse holds the necessary data from the response for creating a new file in a github repo
type CreateResponse struct {
	Content struct {
		SHA string `json:"sha"`
	} `json:"content"`
}

// UploadFile uploads the file to the github repo specified by the url
// creates a file if it does not exist (sha==""), updates it otherwise
func UploadFile(url string, client *http.Client, sha string) (CreateResponse, error) {
	var nilCreateResponse CreateResponse
	// create a commit message and initial content
	// the value for the content is the base64 encoded text "intial content"
	var content string
	var message string
	if sha == "" {
		content = "bXkgbmV3IGZpbGUgY29udGVudHM"
		message = "creating file to be uploaded"
	} else {
		// the content will be unique,
		content = base64.StdEncoding.EncodeToString([]byte(sha))
		message = fmt.Sprintf("updating file with sha:%s", sha)
	}
	reqBody, err := json.Marshal(map[string]string{
		"message": message,
		"content": content,
		"sha":     sha,
	})
	if err != nil {
		return nilCreateResponse, fmt.Errorf("Error marshalling data into request body: %v", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nilCreateResponse, fmt.Errorf("Error creating PUT request to create file: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %v", os.Getenv("GITHUB_API_TOKEN")))

	resp, err := client.Do(req)
	if err != nil {
		return nilCreateResponse, fmt.Errorf("Error sending PUT request to %v: %v", url, err)
	}
	defer resp.Body.Close()

	var createResponse CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResponse); err != nil {
		return nilCreateResponse, fmt.Errorf("Error decoding resp.body into CreateResponse struct: %v", err)
	}

	return createResponse, nil

}

// UpdateFile accesses the specified file in the specified repo, where the user must have authorization to read and write to the repo. For this reason, it is
// recommended that the repo owned by the user, created solely for the purpose of this contribution cron task
// TODO UpdateFile then commits multiple changes and finally pushes the changes
func UpdateFile(repoName string, pathToFile string, client *http.Client) error {
	// first, we need to get the the sha of the specified file
	// for ease and efficiency, I will be accessing and updating a simple .txt file
	url := fmt.Sprintf("https://api.github.com/repos/%v/%v/contents/%v", os.Getenv("GITHUB_USERNAME"), repoName, pathToFile)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("Error creating GET http request to %v: %v", url, err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %v", os.Getenv("GITHUB_API_TOKEN")))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error in sending GET request to %v: %v", url, err)
	}
	defer resp.Body.Close()

	var response FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("Error decoding response into FileResponse: %v", err)
	}
	var sha string
	if response.Message == "Not Found" {
		// if the file does not exist, create it
		sha = ""
	} else {
		// otherwise, set the sha equal to the sha from the response so that we will update it
		sha = response.SHA
	}
	randomNumber := rand.Intn(5) + 4
	for i := 0; i < randomNumber; i++ {
		createResponse, err := UploadFile(url, client, sha)
		if err != nil {
			return err
		}
		sha = createResponse.Content.SHA
	}
	return nil
}
