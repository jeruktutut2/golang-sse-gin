// https://gist.github.com/mirzaakhena/0936851fd2812c74da74940eaab746d7
package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type NotificationCenter struct {
	subscriberMessageChannelsID map[string]chan interface{}
	subscribersMu               *sync.Mutex
}

func NewNotificationCenter() *NotificationCenter {
	return &NotificationCenter{
		subscriberMessageChannelsID: make(map[string]chan interface{}),
		subscribersMu:               &sync.Mutex{},
	}
}

func (nc *NotificationCenter) Subscribe(id string) error {
	fmt.Printf(">>>>> subscribe %s\n", id)

	nc.subscribersMu.Lock()
	defer nc.subscribersMu.Unlock()

	if _, exist := nc.subscriberMessageChannelsID[id]; exist {
		return fmt.Errorf("outlet %s already registered", id)
	}
	nc.subscriberMessageChannelsID[id] = make(chan interface{})
	return nil
}

func (nc *NotificationCenter) Unsubscribe(id string) error {
	fmt.Printf(">>>>> unsubscribe %s\n", id)

	nc.subscribersMu.Lock()
	defer nc.subscribersMu.Unlock()

	if _, exist := nc.subscriberMessageChannelsID[id]; exist {
		return fmt.Errorf("outlet %s not registered yet", id)
	}
	close(nc.subscriberMessageChannelsID[id])
	delete(nc.subscriberMessageChannelsID, id)
	return nil
}

func (nc *NotificationCenter) Notify(id string, message interface{}) error {
	fmt.Printf(">>>>> send message to %s\n", id)
	nc.subscribersMu.Lock()
	defer nc.subscribersMu.Unlock()

	if _, exist := nc.subscriberMessageChannelsID[id]; !exist {
		return fmt.Errorf("outlet %s is not recognise", id)
	}

	nc.subscriberMessageChannelsID[id] <- message
	return nil
}

func (nc *NotificationCenter) WaitForMessage(id string) <-chan interface{} {
	return nc.subscriberMessageChannelsID[id]
}

func handleSSE(nc *NotificationCenter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.Param("id")

		if err := nc.Subscribe(id); err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]interface{}{"message": err.Error()})
			return
		}

		defer func() {
			if err := nc.Unsubscribe(id); err != nil {
				ctx.JSON(http.StatusBadRequest, map[string]interface{}{"message": err.Error()})
				return
			}
			message := fmt.Sprintf("id %s close connection", id)
			ctx.JSON(http.StatusBadRequest, map[string]interface{}{"message": message})
		}()

		for {
			select {
			case message := <-nc.WaitForMessage(id):
				ctx.SSEvent("message", message)
				ctx.Writer.Flush()

			case <-ctx.Request.Context().Done():
				return
			}
		}
	}
}

func messagehandler(nc *NotificationCenter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.Param("id")
		message := fmt.Sprint("Hello %s", id)

		if err := nc.Notify(id, message); err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]interface{}{"message": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, map[string]interface{}{"message": fmt.Sprintf("message send to %s", ctx.Param("id"))})
	}
}

func main() {
	r := gin.Default()

	nc := NewNotificationCenter()

	r.GET("/message/:id", messagehandler(nc))
	r.GET("/handshake/:id", handleSSE(nc))
	r.Run(":8181")
}
