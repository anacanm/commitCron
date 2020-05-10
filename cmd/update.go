package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// FileResponse holds the necessary data from the response for GETting a file
type FileResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
	Message  string `json:"message"`
}

// UpdateFilesAndCreateRemaining takes a slice of RepoContents and the number of changes it is supposed to make (the capacity),
// and if len(contents) < nRequiredChanges, creates
func UpdateFilesAndCreateRemaining(contents []RepoContent, client *http.Client, errorChan chan error, doneChan chan struct{}) {
	// while there are less contents than than need to be made, we need to create new contents
	// if the len(contents) == cap(contents) (remember: contents was initialized with the numberOfContributions as its capacity), then this will never execute
	for i := len(contents); len(contents) < cap(contents); i++ {
	NameChange:
		// we need to generate a new file name that is unique, so an easy way of doing this is by creating a file name based off of the current specific time
		// the string replaces are performed to remove characters from the string representation of time that are not allowed as file names https://stackoverflow.com/questions/4814040/allowed-characters-in-filename
		newFileName := strings.ReplaceAll(strings.ReplaceAll(time.Now().String(), ":", "x"), ".", ",") + ".go"
		// although it is very, very unlikely that a filename exists in the repo with this name, it is still a non-0 chance, so it must be properly addressed

		for _, v := range contents {
			// we will check the specific path of each file, since we can have duplicate names so long as the two files are in different subdirectories
			// and since we will be inserting new files into the root directory of a repo, the file names only need to be unique to other file names in the root directory
			if newFileName == v.Path {
				// if the name found a duplicate, we need to change the name again
				goto NameChange
			}
		}
		// if this is reached, then the filename is accepted, so we can create a new file to be changed. An empty string for a SHA indicates to
		contents = append(contents, RepoContent{Name: newFileName, Path: newFileName, SHA: "", Type: "file"})
	}

	for _, v := range contents {
		// fmt.Printf("%#v\n\n", v)

		UploadFile(fmt.Sprintf("https://api.github.com/repos/%v/%v/contents/%v", os.Getenv("GITHUB_USERNAME"), os.Getenv("REPO_NAME"), v.Path), client, v.Name, v.SHA, errorChan, doneChan)

	}
}

// UploadFile uploads the file to the github repo specified by the url
// creates a file if it does not exist (sha==""), updates it otherwise
func UploadFile(url string, client *http.Client, fileName string, sha string, errorChan chan error, done chan struct{}) {
	client = &http.Client{
		Timeout: time.Second * 7,
	}
	// create a commit message and initial content
	// the "//" is inserted so that script files can be uploaded (works for languages that have // comments, I may add support for other types of comments)
	var content string
	var message string
	if sha == "" {
		// the value for the content if the file does not exist is the base64 encoded text "// intial contents"
		content = base64.StdEncoding.EncodeToString([]byte("// " + fileName))
		message = "creating file to be uploaded"
	} else {
		// the content will be unique using the previous sha. it is encoded to base64 in compliance with github api's requirement
		content = base64.StdEncoding.EncodeToString([]byte("// " + sha))
		message = fmt.Sprintf("updating file with sha: %v", sha)
	}
	reqBody, err := json.Marshal(map[string]string{
		"message": message,
		"content": content,
		"sha":     sha,
	})
	if err != nil {
		errorChan <- fmt.Errorf("Error marshalling data into request body: %v", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(reqBody))
	if err != nil {
		errorChan <- fmt.Errorf("Error creating PUT request to create file: %v", err)
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %v", os.Getenv("GITHUB_API_TOKEN")))

	resp, err := client.Do(req)
	if err != nil {
		errorChan <- fmt.Errorf("Error sending PUT request to %v: %v", url, err)
		return
	}

	// d, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	errorChan <- err
	// 	return
	// }
	// fmt.Println(string(d) + "\n\n\n")
	// data, err := json.MarshalIndent(resp.Body, "", "	")
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// fmt.Println(string(data))
	resp.Body.Close()
	done <- struct{}{}
}
