package handlers

import (
	"context"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

type CalendarClient struct {
	calendar.Service
}

func NewCalendarClient(clientID string, secret string) *CalendarClient {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: secret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{calendar.CalendarScope},
	}

	ctx := context.Background()

	oauthClient := newOAuthClient(ctx, config)

	svc, err := calendar.New(oauthClient)
	if err != nil {
		log.Fatalf("Unable to create Calendar service: %v", err)
	}
	return &CalendarClient{
		Service: *svc,
	}
}

const (
	NewYorkTimeZone              = "America/New_York"
	DefaultEstimateEventDuration = time.Minute * 30
)

func (c *CalendarClient) CreateCalendarForBoardIfNotExist(board *Board) (*calendar.Calendar, error) {
	cal := &calendar.Calendar{}

	calendarList, err := c.CalendarList.List().Do()
	if err != nil {
		return cal, fmt.Errorf("Unable to retrieve list of calendars: %v", err)
	}

	var calendarID string
	for _, calendarItem := range calendarList.Items {
		// a calendar is deemed created if the calendar description is the boardID
		if calendarItem.Description == board.ID {
			calendarID = calendarItem.Id
		}
	}

	if calendarID == "" {
		cal = &calendar.Calendar{
			Description: board.ID,
			Summary:     board.Name,
			TimeZone:    NewYorkTimeZone,
		}
		cal, err = c.Calendars.Insert(cal).Do()
		if err != nil {
			return cal, fmt.Errorf("issue creating new calendar %s %v", board.Name, err)
		}
	} else {
		cal, err = c.Calendars.Get(calendarID).Do()
		if err != nil {
			return cal, fmt.Errorf("issue getting calendarID %s %v", calendarID, err)
		}
	}

	return cal, nil
}

func (c *CalendarClient) SyncTasksToCalendar(board *Board, cal *calendar.Calendar) (map[int]*calendar.Events, error) {
	// current time rounded down to the begining of the day
	loc, _ := time.LoadLocation(NewYorkTimeZone)
	currentTime := time.Now()
	currentDateTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, loc)

	currentWeekday := currentDateTime.Weekday()

	weekdayDatetime := make(map[int]time.Time)
	weekdayDatetime[int(currentWeekday)] = currentDateTime

	for i := 0; i <= 6; i++ {
		if int(currentWeekday) == i {
			continue
		}
		diff := i - int(currentWeekday)
		duration := time.Hour * 24 * time.Duration(diff)

		weekdayTime := currentDateTime.Add(duration)

		weekdayDatetime[int(weekdayTime.Weekday())] = weekdayTime
	}

	// get events from every day this week
	allEvents := make(map[int]*calendar.Events)
	for k, v := range weekdayDatetime {
		endOfDay := v.Add(time.Hour*23 + time.Minute*59)

		events, err := c.Events.List(cal.Id).TimeMin(v.Format(time.RFC3339)).TimeMax(endOfDay.Format(time.RFC3339)).Do()
		if err != nil {
			return allEvents, fmt.Errorf("issue getting events: %v", err)
		}

		allEvents[k] = events
	}

	var days = map[string]int{
		"Sunday":    0,
		"Monday":    1,
		"Tuesday":   2,
		"Wednesday": 3,
		"Thursday":  4,
		"Friday":    5,
		"Saturday":  6,
	}

	// add tasks as events that are missing
	var eventsToAdd []*calendar.Event
	for _, group := range board.Groups {
		weekdayInt := days[group.Title]

		for _, task := range group.Items {
			taskExistsAsEvent := false

			for _, event := range allEvents[weekdayInt].Items {
				if event.Summary == task.Name {
					taskExistsAsEvent = true
					break
				}
			}

			if !taskExistsAsEvent {
				eventToAdd, err := taskToEvent(&task, weekdayDatetime[weekdayInt])
				if err != nil {
					return allEvents, fmt.Errorf("error converting task to event: %v", err)
				}
				eventsToAdd = append(eventsToAdd, eventToAdd)
			}
		}
	}

	for _, event := range eventsToAdd {
		_, err := c.Events.Insert(cal.Id, event).Do()
		if err != nil {
			return allEvents, fmt.Errorf("issue creating events %s: %v", event.Summary, err)
		}
	}

	// remove events that no longer exist as tasks
	var eventsToRemove []*calendar.Event
	for _, group := range board.Groups {
		weekdayInt := days[group.Title]

		for _, event := range allEvents[weekdayInt].Items {
			eventIsStillTask := false

			for _, task := range group.Items {
				if event.Summary == task.Name {
					eventIsStillTask = true
					break
				}
			}

			if !eventIsStillTask {
				eventsToRemove = append(eventsToRemove, event)
			}
		}
	}

	for _, event := range eventsToRemove {
		err := c.Events.Delete(cal.Id, event.Id).Do()
		if err != nil {
			return allEvents, fmt.Errorf("issue deleting event %s: %v", event.Summary, err)
		}
	}

	// monday.com items due date column overwrites gcal end time
	var eventsToUpdate []*calendar.Event
	for _, group := range board.Groups {
		weekdayInt := days[group.Title]

		for _, event := range allEvents[weekdayInt].Items {

			for _, task := range group.Items {
				if event.Summary == task.Name {
					shouldUpdateEvent, err := eventNeedsToBeUpdated(&task, event)
					if err != nil {
						return allEvents, fmt.Errorf("error checking if eventNeedsToBeUpdated: %v", err)
					}

					if shouldUpdateEvent {
						eventToBeUpdated, err := taskToEvent(&task, weekdayDatetime[weekdayInt])
						if err != nil {
							return allEvents, fmt.Errorf("error converting task to event: %v", err)
						}
						eventToBeUpdated.Id = event.Id
						eventsToUpdate = append(eventsToUpdate, eventToBeUpdated)
					}
				}
			}
		}
	}

	for _, event := range eventsToUpdate {
		_, err := c.Events.Update(cal.Id, event.Id, event).Do()
		if err != nil {
			panic(fmt.Sprintf("issue updating event %s : %v", event.Summary, err))
		}
	}

	return allEvents, nil
}

func eventNeedsToBeUpdated(task *Item, event *calendar.Event) (bool, error) {
	var taskDueDate time.Time
	var taskEstimate time.Duration

	for _, columnValue := range task.ColumnValues {
		var err error

		if columnValue.Title == EstimateHours {
			taskEstimate, err = time.ParseDuration(*columnValue.Text + "h")
			if err != nil {
				return false, fmt.Errorf("issue converting EstimateHours: %v", err)
			}
		}

		if columnValue.Title == DueDateAndTime {
			if *columnValue.Text != "" {
				loc, _ := time.LoadLocation(NewYorkTimeZone)
				taskDueDate, err = time.ParseInLocation(DueDateAndTimeFormat, *columnValue.Text, loc)
				if err != nil {
					return false, fmt.Errorf("issue parsing DueDateAndTime: %v", err)
				}
			}
		}
	}

	var eventEndDateTime time.Time
	var eventStartDateTime time.Time
	var eventDuration time.Duration

	loc, _ := time.LoadLocation(NewYorkTimeZone)
	eventEndDateTime, err := time.ParseInLocation(time.RFC3339, event.End.DateTime, loc)
	if err != nil {
		return false, fmt.Errorf("issue parsing event end datetime: %v", err)
	}

	eventStartDateTime, err = time.ParseInLocation(time.RFC3339, event.Start.DateTime, loc)
	if err != nil {
		return false, fmt.Errorf("issue parsing event start datetime: %v", err)
	}

	eventDuration = eventEndDateTime.Sub(eventStartDateTime)

	// if due date doesn't exist on task
	if taskDueDate == *new(time.Time) {
		if eventDuration == taskEstimate {
			return false, nil
		}
	}

	if (eventEndDateTime.Sub(taskDueDate) == 0) && (eventDuration == taskEstimate) {
		return false, nil
	}

	return true, nil
}

func taskToEvent(task *Item, defaultStartDateTime time.Time) (*calendar.Event, error) {
	event := &calendar.Event{}

	estimateEventDuration := DefaultEstimateEventDuration

	defaultEndDateTime := defaultStartDateTime.Add(estimateEventDuration)
	var endDateTime time.Time

	defaultEventStatus := "tentative"
	eventStatus := defaultEventStatus

	for _, columnValue := range task.ColumnValues {
		var err error

		if columnValue.Title == EstimateHours {
			estimateEventDuration, err = time.ParseDuration(*columnValue.Text + "h")
			if err != nil {
				return event, fmt.Errorf("issue converting EstimateHours: %v", err)
			}
		}

		if columnValue.Title == DueDateAndTime {
			if *columnValue.Text != "" {
				loc, _ := time.LoadLocation(NewYorkTimeZone)
				endDateTime, err = time.ParseInLocation(DueDateAndTimeFormat, *columnValue.Text, loc)
				if err != nil {
					return event, fmt.Errorf("issue parsing DueDateAndTime: %v", err)
				}

				if endDateTime.Weekday() != defaultEndDateTime.Weekday() {
					err := fmt.Errorf(
						"the task: '%s' has a due date in Monday.com on the weekday '%s' instead of '%s'."+
							" A task in the group '%s', if it has a due date, should be set to the same day as the group name."+
							" Please fix in Monday.com by removing the due date or changing the date and time.",
						task.Name, endDateTime.Weekday(), defaultEndDateTime.Weekday(), defaultEndDateTime.Weekday())
					return event, err
				}
				eventStatus = "confirmed"
			}
		}
	}

	var startDateTime time.Time
	// if due date not set but estimate is larger than default, ensure event starts at midnight
	if endDateTime == *new(time.Time) {
		startDateTime = defaultStartDateTime
		endDateTime = startDateTime.Add(estimateEventDuration)
	} else {
		startDateTime = endDateTime.Add(-estimateEventDuration)
	}

	event = &calendar.Event{
		Description: "Created by cli tool",
		End: &calendar.EventDateTime{
			DateTime: endDateTime.Format(time.RFC3339),
			TimeZone: NewYorkTimeZone,
		},
		Start: &calendar.EventDateTime{
			DateTime: startDateTime.Format(time.RFC3339),
			TimeZone: NewYorkTimeZone,
		},
		Summary: task.Name,
		Status:  eventStatus,
	}

	return event, nil
}

func osUserCacheDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Caches")
	case "linux", "freebsd":
		return filepath.Join(os.Getenv("HOME"), ".cache")
	}
	log.Printf("TODO: osUserCacheDir on GOOS %q", runtime.GOOS)
	return "."
}

func tokenCacheFile(config *oauth2.Config) string {
	hash := fnv.New32a()
	hash.Write([]byte(config.ClientID))
	hash.Write([]byte(config.ClientSecret))
	hash.Write([]byte(strings.Join(config.Scopes, " ")))
	fn := fmt.Sprintf("go-api-demo-tok%v", hash.Sum32())
	return filepath.Join(osUserCacheDir(), url.QueryEscape(fn))
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := new(oauth2.Token)
	err = gob.NewDecoder(f).Decode(t)
	return t, err
}

func saveToken(file string, token *oauth2.Token) {
	f, err := os.Create(file)
	if err != nil {
		log.Printf("Warning: failed to cache oauth token: %v", err)
		return
	}
	defer f.Close()
	gob.NewEncoder(f).Encode(token)
}

func newOAuthClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile := tokenCacheFile(config)
	token, err := tokenFromFile(cacheFile)
	if err != nil {
		token = tokenFromWeb(ctx, config)
		saveToken(cacheFile, token)
	} else {
		log.Printf("Using cached token %#v from %q", token, cacheFile)
	}

	return config.Client(ctx, token)
}

type customServer struct {
	http.Server
	wg sync.WaitGroup
}

func tokenFromWeb(ctx context.Context, config *oauth2.Config) *oauth2.Token {
	ch := make(chan string)
	randState := fmt.Sprintf("st%d", time.Now().UnixNano())

	// rewritten to use http server instead of httptest because I wanted to hardcode the port on the redirect
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/favicon.ico" {
			http.Error(rw, "", 404)
			return
		}
		if req.FormValue("state") != randState {
			log.Printf("State doesn't match: req = %#v", req)
			http.Error(rw, "", 500)
			return
		}
		if code := req.FormValue("code"); code != "" {
			fmt.Fprintf(rw, "<h1>Success</h1>Authorized.")
			rw.(http.Flusher).Flush()
			ch <- code
			return
		}
		log.Printf("no code")
		http.Error(rw, "", 500)
	})
	server := &customServer{
		Server: http.Server{
			Addr:    "localhost:8080",
			Handler: handler,
		},
	}
	defer server.Close()

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		ln, err := net.Listen("tcp", server.Addr)
		if err != nil {
			panic(err)
		}
		server.Serve(ln)
	}()

	config.RedirectURL = "http://" + server.Addr
	authURL := config.AuthCodeURL(randState)
	go openURL(authURL)
	log.Printf("Authorize this app at: %s", authURL)
	code := <-ch
	log.Printf("Got code: %s", code)

	token, err := config.Exchange(ctx, code)
	if err != nil {
		log.Fatalf("Token exchange error: %v", err)
	}
	return token
}

func openURL(url string) {
	try := []string{"xdg-open", "google-chrome", "open"}
	for _, bin := range try {
		err := exec.Command(bin, url).Run()
		if err == nil {
			return
		}
	}
	log.Printf("Error opening URL in browser.")
}
