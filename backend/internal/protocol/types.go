package protocol

import "encoding/json"

const Version = 1

type Request struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Action  string          `json:"action"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Message struct {
	Version int    `json:"version"`
	ID      string `json:"id,omitempty"`
	Event   string `json:"event"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ExportChatParams struct {
	Chat       string `json:"chat"`
	OutputPath string `json:"outputPath"`
}

type DownloadParams struct {
	ExportPath string   `json:"exportPath"`
	Directory  string   `json:"directory"`
	Filter     string   `json:"filter,omitempty"`
	Include    []string `json:"include,omitempty"`
	Takeout    bool     `json:"takeout,omitempty"`
	Continue   bool     `json:"continue,omitempty"`
	Group      bool     `json:"group,omitempty"`
	SkipSame   bool     `json:"skipSame,omitempty"`
}

type ChatSelection struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}

type StartChatDownloadParams struct {
	Chat            ChatSelection `json:"chat"`
	DestinationRoot string        `json:"destinationRoot"`
	Filter          string        `json:"filter,omitempty"`
	Takeout         bool          `json:"takeout,omitempty"`
	Group           bool          `json:"group,omitempty"`
	SkipSame        bool          `json:"skipSame,omitempty"`
}
