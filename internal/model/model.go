package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func UUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

type Media struct {
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Size     int64   `json:"size"`
	Audio    bool    `json:"audio"`
}
type Manifest struct {
	Objects map[string]string `json:"objects"`
	Input   Media             `json:"input"`
	Preview Media             `json:"preview"`
	Sizes   map[string]int64  `json:"sizes"`
}
type Attempt struct {
	Number        int        `json:"number"`
	Token         string     `json:"token"`
	Worker        string     `json:"worker"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at"`
	Outcome       string     `json:"outcome"`
	ErrorCategory *string    `json:"error_category"`
	ErrorMessage  *string    `json:"error_message"`
	Prefix        string     `json:"output_prefix"`
}
type Job struct {
	ID            string            `json:"id"`
	Filename      string            `json:"filename"`
	State         string            `json:"state"`
	AttemptCount  int               `json:"attempt_count"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CompletedAt   *time.Time        `json:"completed_at"`
	ErrorCategory *string           `json:"error_category"`
	ErrorMessage  *string           `json:"error_message"`
	Manifest      *Manifest         `json:"metadata,omitempty"`
	Outputs       map[string]string `json:"outputs,omitempty"`
	Attempts      []Attempt         `json:"attempts"`
	InputKey      string            `json:"-"`
	InputHash     string            `json:"-"`
	Token         string            `json:"-"`
}
