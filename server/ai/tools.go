package ai

import (
	"fmt"

	"github.com/pkg/errors"
)

type Tool struct {
	Name        string
	Description string
	Schema      any
}

type LookupMattermostUserArgs struct {
	Username string `jsonschema_description:"The username of the user to lookup witout a leading '@'. Example: 'chris.speller'"`
}

var BuiltInTools = []Tool{
	{
		Name:        "LookupMattermostUser",
		Description: "Lookup a Mattermost user by their username.",
		Schema:      LookupMattermostUserArgs{},
	},
}

type ArgumentGetter func(args any) error

func ResolveTool(name string, argsGetter ArgumentGetter) (string, error) {
	switch name {
	case "LookupMattermostUser":
		var args LookupMattermostUserArgs
		err := argsGetter(&args)
		if err != nil {
			return "", errors.Wrap(err, "failed to get arguments for tool "+name)
		}
		fmt.Println(args)
		return "Name: Bob Steven\nEmail:bob@stevenson.com\nUsername: bob.stevenson\nRole: Software Developer 4\nNumber of Eggs: 7", nil
	default:
		return "", errors.New("unknown tool " + name)
	}
}
