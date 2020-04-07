package main

import (
	"fmt"
	"log"

	"github.com/sebradloff/monday-gcal-integration/handlers"
	"google.golang.org/api/calendar/v3"
)

func main() {
	c := LoadConfig(true)
	fmt.Println(c)

	googleClient := handlers.NewGoogleClient(c.ClientID, c.Secret)

	svc, err := calendar.New(googleClient)
	if err != nil {
		log.Fatalf("Unable to create Calendar service: %v", err)
	}

	listRes, err := svc.CalendarList.List().Do()
	if err != nil {
		log.Fatalf("Unable to retrieve list of calendars: %v", err)
	}

	for _, v := range listRes.Items {
		log.Printf("Calendar ID: %v\n", v.Id)
		log.Printf("Calendar summary: %v\n", v.Summary)
		log.Printf("Calendar description: %v\n", v.Description)
	}
}
