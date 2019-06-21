package calendar

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"msclnd/auth"
	"net/http"
	"time"
)

const (
	UA = "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 10.0; WOW64; Trident/7.0; .NET4.0C; .NET4.0E; .NET CLR 2.0.50727; .NET CLR 3.0.30729; .NET CLR 3.5.30729; Microsoft Outlook 16.0.9126; Microsoft Outlook 16.0.9126; ms-office; MSOffice 16)"
)

type Calendar struct {
	client *http.Client
	a      *auth.Auth
}

type Event struct {
	Id         string
	Subject    string
	Start, End *time.Time
	Organizer  string
}

func New(client *http.Client) *Calendar {
	return &Calendar{client: client}
}

type response struct {
	Values []struct {
		Id      string `json:"id"`
		Subject string `json:"subject"`
		Start   struct {
			DateTime string `json:"dateTime"`
			TimeZone string `json:"timeZone"`
		} `json:"start"`
		End struct {
			DateTime string `json:"dateTime"`
			TimeZone string `json:"timeZone"`
		} `json:"end"`
		Organizer struct {
			EmailAddress struct {
				Name    string `json:"name"`
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"organizer"`
	} `json:"value"`
}

const TimeFormat = "2006-01-02T15:04:05"

var est time.Location

func parseWithTz(timeString, tz string) *time.Time {
	t, err := time.Parse(TimeFormat, timeString)
	if err != nil {
		panic(err)
	}
	t = t.In(&est)
	return &t
}

func (c *Calendar) GetMyEvents(tokens *auth.Tokens, start, end time.Time) ([]Event, error) {
	req, err := http.NewRequest(http.MethodGet, "https://graph.microsoft.com/v1.0/me/calendarview", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("User-Agent", UA)
	q := req.URL.Query()
	q.Add("StartDateTime", start.Format(time.RFC3339))
	q.Add("EndDateTime", end.Format(time.RFC3339))
	q.Add("$select", "id,organizer,subject,start,end")

	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		content, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("Can't process URL: %s : [%d = %s]\n%s\n", req.URL.RequestURI(), resp.StatusCode, resp.Status, string(content))
	}

	var r response

	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	evts := make([]Event, len(r.Values))

	for i, v := range r.Values {
		evts[i] = Event{
			Id:        v.Id,
			Organizer: v.Organizer.EmailAddress.Name,
			Subject:   v.Subject,
			Start:     parseWithTz(v.Start.DateTime, v.Start.TimeZone),
			End:       parseWithTz(v.End.DateTime, v.End.TimeZone),
		}
	}

	return evts, nil
}

func init() {
	location, err := time.LoadLocation("EST")
	if err != nil {
		panic(err)
	}
	est = *location
}
