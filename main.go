package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/apps/appclient"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
)

type weatherResponseStruct struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

var (
	weatherResponseData = &weatherResponseStruct{
		ResponseType: "in_channel",
		Text: `---
#### Weather in Toronto, Ontario for the Week of February 16th, 2016

| Day                 | Description                      | High   | Low    |
|:--------------------|:---------------------------------|:-------|:-------|
| Monday, Feb. 15     | Cloudy with a chance of flurries | 3 °C   | -12 °C |
| Tuesday, Feb. 16    | Sunny                            | 4 °C   | -8 °C  |
| Wednesday, Feb. 17  | Partly cloudly                   | 4 °C   | -14 °C |
| Thursday, Feb. 18   | Cloudy with a chance of rain     | 2 °C   | -13 °C |
| Friday, Feb. 19     | Overcast                         | 5 °C   | -7 °C  |
| Saturday, Feb. 20   | Sunny with cloudy patches        | 7 °C   | -4 °C  |
| Sunday, Feb. 21     | Partly cloudy                    | 6 °C   | -9 °C  |
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
			// apps.LocationChannelHeader,
			apps.LocationCommand,
		},
		Deploy: apps.Deploy{
			HTTP: &apps.HTTP{
				RootURL: "http://localhost:4000",
			},
		},
	}
	appBindings = []apps.Binding{
		// {
		//	Location: apps.LocationChannelHeader,
		//	Bindings: []apps.Binding{
		//		{
		//			Location: "send-button",
		//			Icon:     "icon.png",
		//			Label:    "send hello message",
		//			Submit: &apps.Call{
		//				Path: "/send-modal",
		//			},
		//		},
		//	},
		// },
		{
			Location: apps.LocationCommand,
			Bindings: []apps.Binding{
				{
					Label:       "helloworld",
					Description: "Hello World app",
					// Icon:        "icon.png",
					Hint: "[send]",
					Bindings: []apps.Binding{
						{
							Location: "send",
							Label:    "send",
							Submit: &apps.Call{
								Path: "/send",
							},
						},
					},
				},
			},
		},
	}
	// sendForm = apps.Form{
	//	Title: "Hello, world!",
	//	Icon:  "icon.png",
	//	Fields: []apps.Field{
	//		{
	//			Type:  apps.FieldTypeText,
	//			Name:  "message",
	//			Label: "message",
	//		},
	//	},
	//	Submit: &apps.Call{
	//		Path: "/send",
	//	},
	// }
)

func send(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Printf("send(): request body=%s\n", string(bodyBytes))
	callRequest := new(apps.CallRequest)
	err = json.Unmarshal(bodyBytes, callRequest)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	message := "Hello, World!"
	// messageValue, ok := callRequest.Values["message"]
	// if ok && messageValue != nil {
	//	message += fmt.Sprintf(" ...and %s!", messageValue)
	// }
	botClient := appclient.AsBot(callRequest.Context)
	if botClient == nil {
		log.Println("bot client is nil; this is unexpected")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("bot client is nil; this is unexpected"))
		return
	}
	_, err = botClient.DM(callRequest.Context.ActingUserID, message)
	if err != nil {
		log.Printf("error sending message: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	callResponse := apps.NewDataResponse("a post was created in your DM")
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

func makeCallResponse(data interface{}) *apps.CallResponse {
	return &apps.CallResponse{
		Type: apps.CallResponseTypeOK,
		Data: data,
	}
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
	responseBytes, err := json.Marshal(weatherResponseData)
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

func main() {
	serverAddress := "localhost:4000"
	envAddress, ok := os.LookupEnv("SERVER_ADDRESS")
	if ok && envAddress != "" {
		serverAddress = envAddress
		appManifest.Deploy.HTTP.RootURL = fmt.Sprintf("http://%s", serverAddress)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", httputils.DoHandleJSON(appManifest))
	mux.HandleFunc("/bindings", httputils.DoHandleJSON(makeCallResponse(appBindings)))
	mux.HandleFunc("/send", send)
	mux.HandleFunc("/weather", weather)
	server := http.Server{
		Addr:              serverAddress,
		Handler:           mux,
		ReadHeaderTimeout: time.Duration(5) * time.Second,
	}
	log.Printf("Listening on %s\n", serverAddress)
	_ = server.ListenAndServe()
}
