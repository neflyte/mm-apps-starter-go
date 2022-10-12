package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/mattermost/mattermost-plugin-apps/apps/appclient"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
)

type weatherResponseStruct struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

//go:embed static/icon.png
var iconData []byte

var (
	weatherResponseData = &weatherResponseStruct{
		ResponseType: "in_channel",
		Text: `---
![image](/static/icon.png)
#### Weather in Toronto, Ontario for the Week of February 16th, 2016

| Day                 | Description                      | High   | Low    |
|:--------------------|:---------------------------------|:-------|:-------|
| Monday, Feb. 15     | Cloudy with a chance of flurries | 3 °C   | -12 °C |
| Tuesday, Feb. 16    | Sunny                            | 4 °C   | -8 °C  |
| Wednesday, Feb. 17  | Partly cloudy                    | 4 °C   | -14 °C |
| Thursday, Feb. 18   | Cloudy with a chance of rain     | 2 °C   | -13 °C |
| Friday, Feb. 19     | Overcast                         | 5 °C   | -7 °C  |
| Saturday, Feb. 20   | Sunny with cloudy patches        | 7 °C   | -4 °C  |
| Sunday, Feb. 21     | Partly cloudy                    | 6 °C   | -9 °C  |
---`,
	}
	weatherDayResponseData = &weatherResponseStruct{
		ResponseType: "in_channel",
		Text: `---
![image](icon.png)
#### Weather in Toronto, Ontario for Monday, February 15th, 2016

| Day                 | Description                      | High   | Low    |
|:--------------------|:---------------------------------|:-------|:-------|
| Monday, Feb. 15     | Cloudy with a chance of flurries | 3 °C   | -12 °C |
---`,
	}
	appManifest = apps.Manifest{
		AppID:       apps.AppID("hello-world"),
		Version:     apps.AppVersion("0.1.0"),
		HomepageURL: "https://github.com/neflyte/mm-apps-starter-go",
		DisplayName: "Hello, world!",
		Description: "A starter Mattermost App",
		RequestedPermissions: apps.Permissions{
			apps.PermissionActAsBot,
		},
		RequestedLocations: apps.Locations{
			apps.LocationChannelHeader,
			apps.LocationCommand,
			apps.LocationPostMenu,
		},
		Deploy: apps.Deploy{
			HTTP: &apps.HTTP{
				RootURL: "http://localhost:4000",
			},
		},
	}
	appBindings = []apps.Binding{
		{
			Location: apps.LocationChannelHeader,
			Bindings: []apps.Binding{
				{
					Location: "send-button",
					Icon:     "icon.png",
					Label:    "send hello message",
					Submit: &apps.Call{
						Path: "/send-modal",
					},
				},
			},
		},
		{
			Location: apps.LocationCommand,
			Bindings: []apps.Binding{
				{
					Location:    "weather",
					Label:       "weather",
					Description: "Show the weather conditions for today or the next week",
					Hint:        "[day|week]",
					Bindings: []apps.Binding{
						{
							Location:    "day",
							Label:       "day",
							Description: "Show the weather conditions for today",
							Submit: &apps.Call{
								Path: "/weather/day",
							},
						},
						{
							Location:    "week",
							Label:       "week",
							Description: "Show the weather conditions for the next week",
							Submit: &apps.Call{
								Path: "/weather/week",
							},
						},
					},
				},
				{
					Location:    "sub",
					Label:       "sub",
					Hint:        "[event-name] [team-id] [channel-id]",
					Description: "Subscribe to an event",
					Form: &apps.Form{
						Fields: []apps.Field{
							{
								Name:        "eventname",
								Label:       "eventname",
								Type:        apps.FieldTypeText,
								TextSubtype: apps.TextFieldSubtypeInput,
								Description: "The name of the event to subscribe to",
								IsRequired:  true,
							},
							{
								Name:        "teamid",
								Label:       "teamid",
								Type:        apps.FieldTypeText,
								TextSubtype: apps.TextFieldSubtypeInput,
								Description: "The ID of the team",
							},
							{
								Name:        "channelid",
								Label:       "channelid",
								Type:        apps.FieldTypeText,
								TextSubtype: apps.TextFieldSubtypeInput,
								Description: "The ID of the channel",
							},
						},
						Submit: &apps.Call{
							Path: "/sub",
						},
					},
				},
				{
					Location:    "unsub",
					Label:       "unsub",
					Hint:        "[event-name]",
					Description: "Unsubscribe from an event",
					Form: &apps.Form{
						Fields: []apps.Field{
							{
								Name:        "eventname",
								Label:       "eventname",
								Type:        apps.FieldTypeText,
								TextSubtype: apps.TextFieldSubtypeInput,
								Description: "The name of the event to unsubscribe from",
								IsRequired:  true,
							},
						},
						Submit: &apps.Call{
							Path: "/unsub",
						},
					},
				},
			},
		},
		{
			Location: apps.LocationPostMenu,
			Bindings: []apps.Binding{
				{
					Location: "weather",
					Icon:     "icon.png",
					Label:    "Show weather conditions",
					Submit: &apps.Call{
						Path: "/weather",
					},
				},
			},
		},
	}

	subscriptions = make(map[string]*apps.Subscription)
)

func send(w http.ResponseWriter, r *http.Request) {
	requestBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("send(): request=%s\n", string(requestBytes))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callResponse := apps.CallResponse{
		Type: apps.CallResponseTypeOK,
		Text: `### Hello, world!`,
		Data: map[string]interface{}{
			"extra_data": "foo bar baz",
		},
	}
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		log.Printf("error encoding response body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("send(): encodedResponse=%s\n", string(encodedResponse))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func weather(w http.ResponseWriter, r *http.Request) {
	requestBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("weather(): request=%s\n", string(requestBytes))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	var responseBytes []byte
	if strings.HasSuffix(callRequest.Path, "day") {
		responseBytes, err = json.Marshal(weatherDayResponseData)
	} else if strings.HasSuffix(callRequest.Path, "week") {
		responseBytes, err = json.Marshal(weatherResponseData)
	} else {
		responseBytes = []byte(`{"type":"error","text":"unknown argument"}`)
	}
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func subscribeEvent(w http.ResponseWriter, r *http.Request) {
	requestBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("subscribeEvent(): request=%s\n", string(requestBytes))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	// validate parameters
	eventNameIntf, ok := callRequest.Values["eventname"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("event name not specified"))
		return
	}
	eventName, ok := eventNameIntf.(string)
	if !ok || eventName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid event name"))
		return
	}
	channelId := ""
	teamId := ""
	teamIdIntf, ok := callRequest.Values["teamid"]
	if ok {
		teamId = teamIdIntf.(string)
	}
	channelIdIntf, ok := callRequest.Values["channelid"]
	if ok {
		channelId = channelIdIntf.(string)
	}
	// make sure there isn't already a subscription for the one event
	if _, ok = subscriptions[eventName]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("a subscription for this event already exists"))
		return
	}
	// create subscription object and add it to the map
	subscription := &apps.Subscription{
		Event: apps.Event{
			Subject:   apps.Subject(eventName),
			TeamID:    teamId,
			ChannelID: channelId,
		},
		Call: apps.Call{
			Path: "/event",
		},
	}
	subscriptions[eventName] = subscription
	// create a MM client
	clt := appclient.AsBot(callRequest.Context)
	// join a channel if needed
	if channelId != "" {
		_, _, err = clt.AddChannelMember(channelId, callRequest.Context.BotUserID)
		if err != nil {
			err = fmt.Errorf("error adding bot to channel: %w", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
	}
	// subscribe
	err = clt.Subscribe(subscription)
	if err != nil {
		err = fmt.Errorf("error subscribing to event: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	responseBytes := []byte(
		fmt.Sprintf(`{"type":"ok","text":"successfully subscribed to event %s, channel %s, team %s"}`, eventName, channelId, teamId),
	)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func unsubscribeEvent(w http.ResponseWriter, r *http.Request) {
	requestBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("unsubscribeEvent(): request=%s\n", string(requestBytes))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	// validate parameters
	eventNameIntf, ok := callRequest.Values["eventname"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("event name not specified"))
		return
	}
	eventName, ok := eventNameIntf.(string)
	if !ok || eventName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid event name"))
		return
	}
	// Look for a subscription with that event name
	subscription, ok := subscriptions[eventName]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("no subscription for event"))
		return
	}
	// unsubscribe
	clt := appclient.NewClient(callRequest.Context.BotAccessToken, callRequest.Context.MattermostSiteURL)
	err = clt.Unsubscribe(subscription)
	if err != nil {
		err = fmt.Errorf("error unsubscribing from event: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	// remove the subscription from the map
	delete(subscriptions, eventName)
	responseBytes := []byte(
		fmt.Sprintf(`{"type":"ok","text":"successfully unsubscribed from event %s"}`, eventName),
	)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	requestBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("handleEvent(): request=%s\n", string(requestBytes))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	serverAddress := "localhost:4000"
	envAddress, ok := os.LookupEnv("SERVER_ADDRESS")
	if ok && envAddress != "" {
		serverAddress = envAddress
		appManifest.Deploy.HTTP.RootURL = fmt.Sprintf("http://%s", serverAddress)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", httputils.DoHandleJSON(appManifest))
	mux.HandleFunc("/bindings", httputils.DoHandleJSON(apps.NewDataResponse(appBindings)))
	mux.HandleFunc("/send", send)
	mux.HandleFunc("/weather", weather)
	mux.HandleFunc("/weather/day", weather)
	mux.HandleFunc("/weather/week", weather)
	mux.HandleFunc("/sub", subscribeEvent)
	mux.HandleFunc("/unsub", unsubscribeEvent)
	mux.HandleFunc("/event", handleEvent)
	mux.HandleFunc("/static/icon.png", httputils.DoHandleData("image/png", iconData))
	server := http.Server{
		Addr:              serverAddress,
		Handler:           mux,
		ReadHeaderTimeout: time.Duration(5) * time.Second,
	}
	log.Printf("Listening on %s\n", serverAddress)
	_ = server.ListenAndServe()
}
