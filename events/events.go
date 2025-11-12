package events

import "time"

type SubmitEvent struct {
    Username string
    Time     time.Time
}

