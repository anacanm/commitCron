# contributionCron

contributionCron is my solution to having the GitHub public profile contributions be monitored by recruiters (it's rather panoptic).

contributionCron is a script designed to be set up as a daily scheduled task, which makes contributions to a specified repository if a minimum number of contributions has not been met that day.

## Setup

Clone or download this repository, and configure your environment variables. If running locally or on your own dedicated server, using a .env file is recommended. If you are using a remote provider, then that specific service will require their own form of configuration for environment variables.

Create a repository for this script to modify. I highly recommend creating a "burner" repository that serves no purpose other than to be modified by this script. If you want contributions to your repository to show on your contribution graph, then make sure that your repository is public. 

## Environment Variables
contributionCron uses the following environment variables for configuration:

#### GITHUB_USERNAME (required)
the owner (presumably you) of the repository that you will be making contributions to
#### GITHUB_API_TOKEN (required)
Create a token [here](https://github.com/settings/tokens) that will authorize you to make changes to a repo and its contents. For this script to properly work, you need to grant full access to the repo scope when creating the token
#### REPO_NAME (required)
The name of the repository that you wish to modify. Know that you need write access to the repository. 
#### NUMBER_CONTRIBUTIONS (optional)
The number of contributions you would like to make each day. If not specified, will default to a pseudo-random (randomized each day) number between 3 and 7 (inclusive, inclusive).
#### MIN_CONTRIBUTIONS (optional)
The minimum number of contributions to be made each day. If you have already made n contributions on a given day, and n > MIN_CONTRIBUTIONS, then the script will not create any additional contributions. If not specified, will make contributions regardless of the number of contributions already made that day.

## Running the script
As stated before, this script is designed to be run as a daily scheduled task. I recommend running it close to midnight each day if you are specifying a MIN_CONTRIBUTIONS. This is easily attainable using cron or a similar tool. Know that if this is run as a cron task on your machine, it will not run if your computer is powered off when the task is supposed to run. For this reason, I recommend using a free service such as [Heroku Scheduler](https://devcenter.heroku.com/articles/scheduler) that runs on a remote server. Since this script compiles down to a single binary, the task is as simple as executing the binary. 
