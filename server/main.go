package main

import (
	"github.com/mattermost/mattermost/server/public/plugin"
)

var buildHash string

func main() {
	plugin.ClientMain(&Plugin{})
}
