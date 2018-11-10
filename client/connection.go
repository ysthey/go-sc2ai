package client

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/chippydip/go-sc2ai/api"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

type connection struct {
	Status api.Status

	requests chan<- request
}

type request struct {
	data     []byte
	response chan<- response
}

type response struct {
	data []byte
	error
}

// MaxMessageSize is the largest protobuf message that can be sent without getting disconnected.
// The gorilla/websocket implementation fragments messages above it's write buffer size and the
// SC2 game doesn't seem to be able to deal with these messages. There is not a check in place
// to prevent large messages from being sent and warnings will be printed if a message size
// exceeds half of this limit. The default is now 2MB (up from 4kb) but can be overrided by
// modifying this value before connecting to SC2.
var MaxMessageSize = 2 * 1024 * 1024

// Connect ...
func (c *connection) Connect(address string, port int) error {
	c.Status = api.Status_unknown

	dialer := websocket.Dialer{WriteBufferSize: MaxMessageSize}
	url := fmt.Sprintf("ws://%v:%v/sc2api", address, port)

	ws, _, err := dialer.Dial(url, nil)
	if err != nil {
		return err
	}

	requests := make(chan request)
	ws.SetCloseHandler(func(code int, text string) error {
		close(requests)
		return nil
	})
	c.requests = requests

	// Worker
	go func() {
		defer recoverPanic()

		for r := range requests {
			r.process(ws)
		}
	}()

	_, err = c.ping(api.RequestPing{})
	return err
}

func (r request) process(ws *websocket.Conn) {
	data, err := []byte(nil), ws.WriteMessage(websocket.TextMessage, r.data)
	if err == nil {
		_, data, err = ws.ReadMessage()
	}

	r.response <- response{data, err}
	close(r.response)
}

func (c *connection) sendRecv(data []byte, name string) ([]byte, error) {
	out := make(chan response)
	c.requests <- request{data, out}

	for {
		select {
		case r := <-out:
			return r.data, r.error
		case <-time.After(10 * time.Second):
			fmt.Printf("waiting for %v response\n", name)
		}
	}
}

func (c *connection) request(r *api.Request) (*api.Response, error) {
	// Serialize
	data, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}

	if len(data) > MaxMessageSize {
		err = fmt.Errorf("message too large: %v (max %v)", len(data), MaxMessageSize)
		log.Println(os.Stderr, err)
		return nil, err
	} else if len(data) > MaxMessageSize/2 {
		log.Println(os.Stderr, "warning, large message size:", len(data))
	}

	// Send/Recv
	data, err = c.sendRecv(data, reflect.TypeOf(r.Request).String())
	if err != nil {
		return nil, err
	}

	// Deserialize
	resp := &api.Response{}
	err = proto.Unmarshal(data, resp)
	if err != nil {
		return nil, err
	}

	// Update status
	if resp.Status != api.Status_nil {
		c.Status = resp.Status
	}

	// Report errors (if any) and return
	switch len(resp.Error) {
	case 0:
		return resp, nil
	case 1:
		return nil, errors.New(resp.Error[0])
	default:
		return nil, fmt.Errorf("%v", resp.Error)
	}
}
