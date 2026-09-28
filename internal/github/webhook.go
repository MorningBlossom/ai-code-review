package github

import (
	"time"
)

type WebhookEvent struct {
	EventType      string
	DeliveryID     string
	InstallationID int64
	Organization   string
	Repository     string
	PullRequest    PullRequestEvent
	ReceivedAt     time.Time
}

type PullRequestEvent struct {
	Action  string
	Number  int
	Title   string
	Body    string
	BaseSHA string
	HeadSHA string
	Author  string
}
