package calendar

import (
	"encoding/json"
	"fmt"
	"log"
	"msclnd/auth"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

type GoogleCal struct {
	calendarId string
	tokenFile  string
	svc        *calendar.Service
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config, tokFile string) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tok, tokFile)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(tokenFile string) (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(token *oauth2.Token, tokenFile string) {
	fmt.Printf("Saving credential file to: %s\n", tokenFile)
	f, err := os.OpenFile(tokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

type insertErrors struct {
	Events []Event
	Errors []error
}

func (ie *insertErrors) Error() string {
	return fmt.Sprintf("Can't proceed with the errors: %+v", ie)
}

func IsInsertError(e error) bool {
	_, ok := e.(*insertErrors)
	return ok
}

func ExtractInsertErrors(e error) ([]Event, []error, error) {
	if ie, ok := e.(*insertErrors); ok {
		return ie.Events, ie.Errors, nil
	} else {
		return nil, nil, errors.New("Not an insert error")
	}
}

func (gc *GoogleCal) AddEvents(events []Event) error {
	addErrors := insertErrors{
		Events: make([]Event, 0),
		Errors: make([]error, 0),
	}
	for _, evt := range events {
		fmt.Printf("Adding event %v\n", evt)
		if _, err := gc.svc.Events.Insert(
			gc.calendarId,
			&calendar.Event{
				Summary: evt.Subject + " by [ " + evt.Organizer + " ]",
				Start: &calendar.EventDateTime{
					DateTime: evt.Start.Format(time.RFC3339),
					TimeZone: "UTC",
				},
				End: &calendar.EventDateTime{
					DateTime: evt.End.Format(time.RFC3339),
					TimeZone: "UTC",
				},
			},
		).Do(); err != nil {
			fmt.Printf("Failed to add event: %+v\n", err)
			addErrors.Events = append(addErrors.Events, evt)
			addErrors.Errors = append(addErrors.Errors, err)
		}
	}
	if len(addErrors.Errors) == 0 {
		return nil
	} else {
		return &addErrors
	}
}

func NewGoogleCal(googleCreds auth.GoogleCalConf, tokenFile string, calendarId string) (*GoogleCal, error) {
	config := &oauth2.Config{
		ClientID:     googleCreds.Installed.ClientID,
		ClientSecret: googleCreds.Installed.ClientSecret,
		RedirectURL:  googleCreds.Installed.RedirectUris[0],
		Scopes:       []string{calendar.CalendarEventsScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  googleCreds.Installed.AuthURI,
			TokenURL: googleCreds.Installed.TokenURI,
		},
	}

	client := getClient(config, tokenFile)

	srv, err := calendar.New(client)
	if err != nil {
		return nil, errors.Wrap(err, "can't create Google calendar instance")
	}
	return &GoogleCal{
		calendarId: calendarId,
		tokenFile:  tokenFile,
		svc:        srv,
	}, nil
}
