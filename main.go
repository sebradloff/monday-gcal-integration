package main

import (
	"fmt"

	"github.com/sebradloff/monday-gcal-integration/handlers"
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

	// ensure all tasks on the board exist on the calendar in the right days
	calendarClient.SyncTasksToCalendar(board, cal)
}
