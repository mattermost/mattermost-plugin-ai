package main

import (
	"fmt"
	"time"
)

type Timer struct {
	start   time.Time
	subject string
}

func NewTimer(subject string) *Timer {
	fmt.Println("Starting " + subject)
	return &Timer{start: time.Now(), subject: subject}
}

func (t *Timer) Finish() {
	fmt.Printf("Done "+t.subject+"! Time taken: %v\n", time.Since(t.start))
}
