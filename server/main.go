package main

import (
	"github.com/mattermost/mattermost/server/public/plugin"
)

var buildHash string
var rudderWriteKey string
var rudderDataplaneURL string

func main() {
	plugin.ClientMain(&Plugin{})
}
