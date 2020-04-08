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

	mondayClient := handlers.NewMondayClient(c.MondayAPIKey)
	// get board from monday.com
	board, err := mondayClient.GetAllItemsInGroupsByBoardId(c.BoardID)
	if err != nil {
		panic(err)
	}
	fmt.Println(board)

	calendarClient := handlers.NewCalendarClient(c.ClientID, c.Secret)

	// if calendar name does not exist, create it
	cal, err := calendarClient.CreateCalendarForBoardIfNotExist(board)
	if err != nil {
		panic(err)
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
