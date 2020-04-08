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

func (c *CalendarClient) CreateCalendarForBoardIfNotExist(board *Board) (*calendar.Calendar, error) {
	cal := &calendar.Calendar{}

	calendarList, err := c.CalendarList.List().Do()
	if err != nil {
		return cal, fmt.Errorf("Unable to retrieve list of calendars: %v", err)
	}

	var calendarID string
	for _, calendarItem := range calendarList.Items {
		// a calendar is deemed created if the description is the boardID
		if calendarItem.Description == board.ID {
			calendarID = calendarItem.Id
		}
	}

	if calendarID == "" {
		cal = &calendar.Calendar{
			Description: board.ID,
			Summary:     board.Name,
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
