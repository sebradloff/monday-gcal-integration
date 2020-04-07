package main

import (
	"context"
	"fmt"
	"log"

	"github.com/machinebox/graphql"
	"github.com/sebradloff/monday-gcal-integration/handlers"
	"google.golang.org/api/calendar/v3"
)

func main() {
	c := LoadConfig(true)
	fmt.Println(c)

	client := graphql.NewClient("https://api.monday.com/v2/")
	req := graphql.NewRequest(`
	query getAllItemsInGroupsByBoardId ($boardID: [Int]) {
		complexity {
			query
			after
			before
		}
	  boards(ids: $boardID) {
		groups{
		  id
		  title
		  items(limit: 50) {
			  id
			  name
			  updated_at
			  column_values {
				  id
				  text
				  title
			  }
		  }
		}
	  }
	}
	`)
	req.Var("boardID", c.BoardID)
	req.Header.Set("Authorization", c.MondayAPIKey)

	// define a Context for the request
	ctx := context.Background()

	// run it and capture the response
	var graphqlResponse interface{}
	if err := client.Run(ctx, req, &graphqlResponse); err != nil {
		panic(err)
	}
	fmt.Println(graphqlResponse)

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
