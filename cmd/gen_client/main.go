package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"strings"
)

// List of request types (because it was easiest to just copy/pasta this from the protobuf code)
const requestTypes = `
//	*Request_CreateGame
//	*Request_JoinGame
//	*Request_RestartGame
//	*Request_StartReplay
//	*Request_LeaveGame
//	*Request_QuickSave
//	*Request_QuickLoad
//	*Request_Quit
//	*Request_GameInfo
//	*Request_Observation
//	*Request_Action
//	*Request_ObsAction
//	*Request_Step
//	*Request_Data
//	*Request_Query
//	*Request_SaveReplay
//	*Request_ReplayInfo
//	*Request_AvailableMaps
//	*Request_SaveMap
//	*Request_Ping
//	*Request_Debug
`

// Code templates
const header = `// Code generated by gen_client. DO NOT EDIT.
package client

import (
	"github.com/chippydip/go-sc2ai/api"
)
`

const methodTemplate = `
func (c *connection) {{.Arg}}({{.Arg}} api.Request{{.Name}}) (*api.Response{{.Name}}, error) {
	r, err := c.request(&api.Request{
		Request: &api.Request_{{.Short}}{
			{{.Short}}: &{{.Arg}},
		},
	})
	return r.Get{{.Short}}(), err
}
`

type methodData struct {
	Name  string
	Short string
	Arg   string
}

func main() {
	t := template.Must(template.New("method").Parse(methodTemplate))

	file, err := os.Create("client/api.go")
	check(err)
	defer file.Close()

	writer := bufio.NewWriter(file)
	fmt.Fprint(writer, header)

	lines := strings.Split(requestTypes, "\n")
	for _, line := range lines {
		parts := strings.Split(line, "_")
		if len(parts) < 2 {
			continue
		}
		name := parts[1]

		d := methodData{
			Name:  name,
			Short: name,
			Arg:   strings.ToLower(name[:1]) + name[1:],
		}

		// This one is a special snowflake
		if d.Short == "ObsAction" {
			d.Name = "ObserverAction"
		}

		t.Execute(writer, d)
	}

	check(writer.Flush())
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
