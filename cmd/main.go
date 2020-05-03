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

	client := &http.Client{
		Timeout: time.Second * 7,
	}
	numberOfContributions, err := contributions.GetNumberOfContributionsToday(client)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n\nNumber of contributions: %v", numberOfContributions)
}
