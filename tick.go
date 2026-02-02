package main

import (
	"log"
	"time"
)

func startTickLoop() {
	ticker := time.NewTicker(60 * time.Second)

	go func() {
		for t := range ticker.C {
			log.Println("Tick:", t.UTC())
		}
	}()
}
