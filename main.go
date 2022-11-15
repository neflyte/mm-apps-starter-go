package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/apps/appclient"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
	"github.com/mattermost/mattermost-server/v6/model"
)

type weatherResponseStruct struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

//go:embed static/icon.png
var iconData []byte

//go:embed static/icon-info.png
var iconInfoData []byte

//go:embed static/icon-head.png
var iconHeadData []byte

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
				RootURL: "http://mm-apps-starter-go:4000",
			},
		},
		OnInstall: apps.NewCall("/installed").WithExpand(apps.Expand{
			App: apps.ExpandSummary,
		}),
		OnUninstall: apps.NewCall("/uninstalled"),
	}

	appBindings = []apps.Binding{
		{
			Location: apps.LocationChannelHeader,
			Bindings: []apps.Binding{
				{
					Location: "send-button",
					Icon:     "icon.png",
					Label:    "send hello message",
					Submit:   apps.NewCall("/send"),
				},
				{
					Location: "info-button",
					Icon:     "icon-info.png",
					Label:    "Dynamic field test",
					Submit:   apps.NewCall("/send-dynamic-form"),
				},
				{
					Location: "message-attachment",
					Icon:     "icon-head.png",
					Label:    "Message attachment test",
					Submit: apps.NewCall("/send-message-attachment").WithExpand(apps.Expand{
						ActingUser: apps.ExpandID,
						Channel:    apps.ExpandID,
					}),
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
							Submit:      apps.NewCall("/weather/day"),
						},
						{
							Location:    "week",
							Label:       "week",
							Description: "Show the weather conditions for the next week",
							Submit:      apps.NewCall("/weather/week"),
						},
					},
				},
				{
					Location:    "sub",
					Label:       "sub",
					Hint:        "[eventname] [teamid] [channelid]",
					Description: "Subscribe to an event",
					Form: &apps.Form{
						Title:  "Subscribe to an event",
						Header: "Subscribe to a Mattermost Server event",
						Icon:   "icon.png",
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
						Submit: apps.NewCall("/sub"),
					},
				},
				{
					Location:    "unsub",
					Label:       "unsub",
					Hint:        "[eventname]",
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
						Submit: apps.NewCall("/unsub"),
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
					Submit:   apps.NewCall("/weather"),
				},
			},
		},
	}

	sendForm = apps.Form{
		Title: "Hello, world!",
		Icon:  "icon.png",
		Source: &apps.Call{
			Path: "/send-form-source",
		},
		Fields: []apps.Field{
			{
				Type:  apps.FieldTypeText,
				Name:  "message",
				Label: "Message",
			},
			{
				Type:          apps.FieldTypeUser,
				Name:          "user",
				Label:         "User",
				SelectRefresh: true,
			},
			{
				Type:  apps.FieldTypeStaticSelect,
				Name:  "option",
				Label: "Option",
				SelectStaticOptions: []apps.SelectOption{
					{
						Label: "Option One",
						Value: "option_1",
					},
					{
						Label: "Option Two",
						Value: "option_2",
					},
				},
			},
		},
		Submit: &apps.Call{
			Path: "/modal-submit",
		},
	}

	dynamicForm = apps.Form{
		Title: "Dynamic field test",
		Icon:  "icon-info.png",
		Fields: []apps.Field{
			{
				Type:  apps.FieldTypeDynamicSelect,
				Name:  "option",
				Label: "Option",
				SelectDynamicLookup: &apps.Call{
					Path: "/dynamic-form-lookup",
				},
			},
		},
		Submit: &apps.Call{
			Path: "/dynamic-form-submit",
		},
	}

	subscriptions = make(map[string]*apps.Subscription)
)

func getCallRequest(r *http.Request) (*apps.CallRequest, error) {
	requestBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		return nil, err
	}
	log.Printf("getCallRequest(): request=%s\n", string(requestBytes))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("getCallRequest(): error decoding request body: %s\n", err.Error())
		return nil, err
	}
	return callRequest, nil
}

func sendErrorResponse(w http.ResponseWriter, err error) {
	errorResponse := apps.NewErrorResponse(err)
	encodedResponse, err := json.Marshal(errorResponse)
	if err != nil {
		log.Printf("sendErrorResponse(): error encoding response body: %s\n", err.Error())
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("sendErrorResponse(): encodedResponse=%s\n", string(encodedResponse))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write(encodedResponse)
}

func sendFormSource(w http.ResponseWriter, r *http.Request) {
	callRequest, err := getCallRequest(r)
	if err != nil {
		log.Printf("sendFormSource(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	userIntf, ok := callRequest.Values["user"]
	if !ok {
		sendErrorResponse(w, errors.New("expected value 'user' is missing"))
		return
	}
	userMap, ok := userIntf.(map[string]interface{})
	if !ok {
		sendErrorResponse(w, errors.New("expected value 'user' is not a map"))
		return
	}
	sendFormClone := sendForm.PartialCopy()
	sendFormClone.Fields[1].Value = userMap
	// return the same form since we don't want to do anything further
	callResponse := apps.CallResponse{
		Type: apps.CallResponseTypeForm,
		Form: sendFormClone,
	}
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	log.Printf("sendFormSource(): encodedResponse=%s\n", string(encodedResponse))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func send(w http.ResponseWriter, r *http.Request) {
	_, err := getCallRequest(r)
	if err != nil {
		log.Printf("send(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	callResponse := apps.CallResponse{
		Type: apps.CallResponseTypeForm,
		Form: &sendForm,
	}
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		log.Printf("error encoding response body: %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	log.Printf("send(): encodedResponse=%s\n", string(encodedResponse))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func weather(w http.ResponseWriter, r *http.Request) {
	callRequest, err := getCallRequest(r)
	if err != nil {
		log.Printf("weather(): %s\n", err.Error())
		sendErrorResponse(w, err)
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
		sendErrorResponse(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func subscribeEvent(w http.ResponseWriter, r *http.Request) {
	callRequest, err := getCallRequest(r)
	if err != nil {
		log.Printf("subscribeEvent(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	// validate parameters
	eventNameIntf, ok := callRequest.Values["eventname"]
	if !ok {
		sendErrorResponse(w, errors.New("event name not specified"))
		return
	}
	eventName, ok := eventNameIntf.(string)
	if !ok || eventName == "" {
		sendErrorResponse(w, errors.New("invalid event name"))
		return
	}
	channelId := ""
	teamId := ""
	teamIdIntf, ok := callRequest.Values["teamid"]
	if ok && teamIdIntf != nil {
		teamId = teamIdIntf.(string)
	}
	channelIdIntf, ok := callRequest.Values["channelid"]
	if ok && channelIdIntf != nil {
		channelId = channelIdIntf.(string)
	}
	// make sure there isn't already a subscription for the one event
	if _, ok = subscriptions[eventName]; ok {
		sendErrorResponse(w, errors.New("a subscription for this event already exists"))
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
			sendErrorResponse(w, err)
			return
		}
	}
	// subscribe
	err = clt.Subscribe(subscription)
	if err != nil {
		err = fmt.Errorf("error subscribing to event: %w", err)
		sendErrorResponse(w, err)
		return
	}
	responseBytes := []byte(
		fmt.Sprintf(`{"type":"ok","text":"successfully subscribed to event %s, channel %s, team %s"}`, eventName, channelId, teamId),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func unsubscribeEvent(w http.ResponseWriter, r *http.Request) {
	callRequest, err := getCallRequest(r)
	if err != nil {
		log.Printf("unsubscribeEvent(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	// validate parameters
	eventNameIntf, ok := callRequest.Values["eventname"]
	if !ok {
		sendErrorResponse(w, errors.New("event name not specified"))
		return
	}
	eventName, ok := eventNameIntf.(string)
	if !ok || eventName == "" {
		sendErrorResponse(w, errors.New("invalid event name"))
		return
	}
	// Look for a subscription with that event name
	subscription, ok := subscriptions[eventName]
	if !ok {
		sendErrorResponse(w, errors.New("no subscription for event"))
		return
	}
	// unsubscribe
	clt := appclient.NewClient(callRequest.Context.BotAccessToken, callRequest.Context.MattermostSiteURL)
	err = clt.Unsubscribe(subscription)
	if err != nil {
		err = fmt.Errorf("error unsubscribing from event: %w", err)
		sendErrorResponse(w, err)
		return
	}
	// remove the subscription from the map
	delete(subscriptions, eventName)
	responseBytes := []byte(
		fmt.Sprintf(`{"type":"ok","text":"successfully unsubscribed from event %s"}`, eventName),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	_, err := getCallRequest(r)
	if err != nil {
		log.Printf("handleEvent(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	responseBytes := []byte(`{"type":"ok","text":"handleEvent() was called"}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func appInstalled(w http.ResponseWriter, r *http.Request) {
	log.Println("app installed")
	responseBytes := []byte(`{"type":"ok","text":"successfully installed app"}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func appUninstalled(w http.ResponseWriter, r *http.Request) {
	log.Println("app uninstalled")
	responseBytes := []byte(`{"type":"ok","text":"successfully uninstalled app"}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func sendDynamicForm(w http.ResponseWriter, r *http.Request) {
	_, err := getCallRequest(r)
	if err != nil {
		log.Printf("sendDynamicForm(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	callResponse := apps.CallResponse{
		Type: apps.CallResponseTypeForm,
		Form: &dynamicForm,
	}
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	log.Printf("sendDynamicForm(): encodedResponse=%s\n", string(encodedResponse))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func dynamicFormLookup(w http.ResponseWriter, r *http.Request) {
	_, err := getCallRequest(r)
	if err != nil {
		log.Printf("dynamicFormLookup(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	callResponse := apps.NewDataResponse(map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"label": "Option One",
				"value": "option_1",
			},
			map[string]interface{}{
				"label": "Option Two",
				"value": "option_2",
			},
		},
	})
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	log.Printf("dynamicFormLookup(): encodedResponse=%s\n", string(encodedResponse))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func modalSubmit(w http.ResponseWriter, r *http.Request) {
	callRequest, err := getCallRequest(r)
	if err != nil {
		log.Printf("modalSubmit(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	responseText := "## Form values\n"
	for key := range callRequest.Values {
		responseText += fmt.Sprintf("- %s: %#v\n", key, callRequest.Values[key])
	}
	callResponse := apps.NewTextResponse(responseText)
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	log.Printf("modalSubmit(): encodedResponse=%s\n", string(encodedResponse))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func sendMessageAttachment(w http.ResponseWriter, r *http.Request) {
	callRequest, err := getCallRequest(r)
	if err != nil {
		log.Printf("sendMessageAttachment(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	post := &model.Post{
		ChannelId: callRequest.Context.Channel.Id,
	}
	postAppBindingsCall := apps.NewCall("/set-roast-preference")
	postAppBindings := []apps.Binding{
		{
			Location:    "embedded",
			AppID:       appManifest.AppID,
			Description: "Select your favourite coffee roast",
			Bindings: []apps.Binding{
				{
					Location: "coffee-roast",
					Label:    "Coffee roast",
					Bindings: []apps.Binding{
						{
							Location: "dark-roast",
							Label:    "Dark roast",
							Submit:   postAppBindingsCall,
						},
						{
							Location: "medium-roast",
							Label:    "Medium roast",
							Submit:   postAppBindingsCall,
						},
						{
							Location: "light-roast",
							Label:    "Light roast",
							Submit:   postAppBindingsCall,
						},
					},
				},
			},
		},
	}
	post.AddProp(apps.PropAppBindings, postAppBindings)
	clt := appclient.AsBot(callRequest.Context)
	_, err = clt.CreatePost(post)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	callResponse := apps.CallResponse{
		Type: apps.CallResponseTypeOK,
	}
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func setRoastPreference(w http.ResponseWriter, r *http.Request) {
	_, err := getCallRequest(r)
	if err != nil {
		log.Printf("setRoastPreference(): %s\n", err.Error())
		sendErrorResponse(w, err)
		return
	}
	callResponse := apps.CallResponse{
		Type: apps.CallResponseTypeOK,
	}
	encodedResponse, err := json.Marshal(callResponse)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encodedResponse)
}

func main() {
	serverAddress := "localhost:4000"
	envAddress, ok := os.LookupEnv("SERVER_ADDRESS")
	if ok && envAddress != "" {
		serverAddress = envAddress
		appManifest.Deploy.HTTP.RootURL = fmt.Sprintf("http://%s", serverAddress)
	}
	mux := httputils.NewHandler()
	mux.HandleFunc("/manifest.json", httputils.DoHandleJSON(appManifest))
	mux.HandleFunc("/bindings", httputils.DoHandleJSON(apps.NewDataResponse(appBindings)))
	mux.HandleFunc("/send", send)
	mux.HandleFunc("/weather", weather)
	mux.HandleFunc("/weather/day", weather)
	mux.HandleFunc("/weather/week", weather)
	mux.HandleFunc("/sub", subscribeEvent)
	mux.HandleFunc("/unsub", unsubscribeEvent)
	mux.HandleFunc("/event", handleEvent)
	mux.HandleFunc("/installed", appInstalled)
	mux.HandleFunc("/uninstalled", appUninstalled)
	mux.HandleFunc("/send-form-source", sendFormSource)
	mux.HandleFunc("/send-dynamic-form", sendDynamicForm)
	mux.HandleFunc("/dynamic-form-lookup", dynamicFormLookup)
	mux.HandleFunc("/modal-submit", modalSubmit)
	mux.HandleFunc("/send-message-attachment", sendMessageAttachment)
	mux.HandleFunc("/set-roast-preference", setRoastPreference)
	mux.HandleFunc("/static/icon.png", httputils.DoHandleData("image/png", iconData))
	mux.HandleFunc("/static/icon-info.png", httputils.DoHandleData("image/png", iconInfoData))
	mux.HandleFunc("/static/icon-head.png", httputils.DoHandleData("image/png", iconHeadData))
	server := http.Server{
		Addr:              serverAddress,
		Handler:           mux,
		ReadHeaderTimeout: time.Duration(5) * time.Second,
	}
	log.Printf("Listening on %s\n", serverAddress)
	_ = server.ListenAndServe()
}
