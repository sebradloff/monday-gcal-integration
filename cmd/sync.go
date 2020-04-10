package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/sebradloff/monday-gcal-integration/handlers"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync [boardID]",
	Short: "To sync tasks for your Monday.com board to a Google Calendar",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("requires a Monday.com boardID")
		}

		if err := boardIDArgValidation(args[0]); err != nil {
			return fmt.Errorf("issue validating boardID: %v", err)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			panic("")
		}

		mondayClient := handlers.NewMondayClient(mondayAPIKey)
		// get board from monday.com
		board, err := mondayClient.GetAllItemsInGroupsByBoardId(boardID)
		if err != nil {
			panic(err)
		}

		calendarClient := handlers.NewCalendarClient(googleClientID, googleSecret)

		// if calendar name does not exist, create it
		cal, err := calendarClient.CreateCalendarForBoardIfNotExist(board)
		if err != nil {
			panic(err)
		}

		// ensure all tasks on the board exist on the calendar in the right days
		_, err = calendarClient.SyncTasksToCalendar(board, cal)
		if err != nil {
			panic(err)
		}

		fmt.Println("done syncing tasks to google calendar")
	},
}

func boardIDArgValidation(arg string) error {
	_, err := strconv.Atoi(arg)
	if err != nil {
		return fmt.Errorf("not a valid int: '%s'", arg)
	}
	return nil
}
