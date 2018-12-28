package webrtc

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
)

func ParseUmsRequest(data []byte) (*UmsRequestJson, error) {
	var jreq UmsRequestJson
	err := json.Unmarshal(data, &jreq)
	return &jreq, err
}

func (r *UmsRequestJson) GetOffer() string {
	log.Println("[json] ums request type:", r.Type, ", session:", r.Action.SessionKey)
	user_roster := r.Action.UserRoster
	if user_roster == nil || len(user_roster) == 0 {
		log.Println("[json] ums no user_roster in json")
		return ""
	}

	channels := user_roster[0].AudioStatus.Channels
	if channels == nil || len(channels) == 0 {
		log.Println("[json] ums no channels in json")
		return ""
	}

	webrtc_offer := channels[0].WebrtcOffer
	return webrtc_offer
}

func ParseUmsResponse(data []byte) (*UmsResponseJson, error) {
	var jreq UmsResponseJson
	err := json.Unmarshal(data, &jreq)
	return &jreq, err
}

func (r *UmsResponseJson) GetAnswer() string {
	log.Println("[json] ums response code:", r.Code)
	user_roster := r.Action.UserRoster
	if user_roster == nil || len(user_roster) == 0 {
		log.Println("[json] ums no user_roster in json")
		return ""
	}

	channels := user_roster[0].AudioStatus.Channels
	if channels == nil || len(channels) == 0 {
		log.Println("[json] ums no channels in json")
		return ""
	}

	webrtc_answer := channels[0].WebrtcAnswer
	return webrtc_answer
}

type UmsChannel struct {
	ChannelId     int    `json:"channel_id,omitempty"`
	WebrtcOffer   string `json:"webrtc_offer"`
	WebrtcServers string `json:"webrtc_servers"`
	WebrtcAnswer  string `json:"webrtc_answer"`
}

type UmsAudioStatus struct {
	Channels []UmsChannel `json:"channels"`
}

type UmsUserRoster struct {
	AudioStatus UmsAudioStatus `json:"audio_status"`
}

type UmsAction struct {
	SessionKey string          `json:"session_key,omitempty"`
	UserRoster []UmsUserRoster `json:"user_roster"`
}

type UmsRequestJson struct {
	Type          string    `json:"type"` // SESSION_REQUEST_WEBRTC_OFFER
	Action        UmsAction `json:"action"`
	MultiConn     bool      `json:"multi_webrtc_conn"`
	Agent         string    `json:"agent"`                 // chrome/firefox
	Version       int       `json:"version"`               // browser version
	WebrtcVersion int       `json:"webrtc_client_version"` // webrtc version
}

type UmsResponseJson struct {
	Action UmsAction `json:"action"`
	Code   string    `json:"code"`
}
