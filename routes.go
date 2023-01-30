package main

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/r3labs/sse/v2"
)

var sseServer *sse.Server

func init() {
	// There are many sse implementations out there,
	// this is just an example of one of them.
	sseServer = sse.New()
	sseServer.AutoStream = true
	sseServer.AutoReplay = true

	streams := make(map[string]int64)
	mu := new(sync.RWMutex)

	sseServer.OnSubscribe = func(streamID string, sub *sse.Subscriber) {
		users.Add("connected", 1)

		mu.Lock()
		streams[streamID] = streams[streamID] + 1
		mu.Unlock()
	}

	sseServer.OnUnsubscribe = func(streamID string, sub *sse.Subscriber) {
		users.Add("disconnected", 1)

		mu.Lock()
		value := streams[streamID] - 1
		if value == 0 {
			delete(streams, streamID)
		} else {
			streams[streamID] = value
		}
		mu.Unlock()
	}

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for range t.C {
			stats := Stats()
			if len(stats) == 0 {
				continue
			}

			data, err := json.Marshal(stats)
			if err != nil {
				return
			}

			streamsCopy := make(map[string]int64)
			mu.RLock()
			for k, v := range streams {
				streamsCopy[k] = v
			}
			mu.RUnlock()

			for streamID := range streamsCopy {
				sseServer.Publish(streamID, &sse.Event{
					Event: []byte("stats"),
					Data:  data,
				})
			}
		}
	}()
}

func rateLimit(ctx iris.Context) {
	ip := ctx.RemoteAddr()
	value := int(ips.Add(ip, 1))
	if value%50 == 0 {
		fmt.Printf("ip: %s, count: %d\n", ip, value)
	}

	if value >= 200 {
		if value%200 == 0 {
			fmt.Println("ip blocked")
		}

		ctx.StopWithText(iris.StatusServiceUnavailable, "you were automatically banned :)")
		return
	}

	ctx.Next()
}

func index(ctx iris.Context) {
	ctx.Redirect("/room/hn", iris.StatusMovedPermanently)
}

func roomGET(ctx iris.Context) {
	roomid := ctx.Params().Get("roomid")
	nick := ctx.URLParam("nick")
	if len(nick) < 2 {
		nick = ""
	}
	if len(nick) > 13 {
		nick = nick[0:12] + "..."
	}

	err := ctx.View("room_login.templ.html", iris.Map{
		"roomid":    roomid,
		"nick":      nick,
		"timestamp": time.Now().Unix(),
	})
	if err != nil {
		ctx.HTML("<strong>%s</strong>", err.Error())
		return
	}
}

func roomPOST(ctx iris.Context) {
	roomid := ctx.Params().Get("roomid")
	nick := ctx.URLParam("nick")
	message := ctx.PostValue("message")
	message = strings.TrimSpace(message)

	validMessage := len(message) > 1 && len(message) < 200
	validNick := len(nick) > 1 && len(nick) < 14
	if !validMessage || !validNick {
		ctx.StopWithJSON(iris.StatusBadRequest, iris.Map{
			"status": "failed",
			"error":  "the message or nickname is too long",
		})
		return
	}

	post := iris.Map{
		"nick":    html.EscapeString(nick),
		"message": html.EscapeString(message),
	}
	messages.Add("inbound", 1)

	data, err := json.Marshal(post)
	if err != nil {
		ctx.StopWithJSON(iris.StatusBadRequest, iris.Map{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	sseServer.Publish(roomid, &sse.Event{
		Event: []byte("message"),
		Data:  data,
	})

	// room(roomid).Submit(post)
	ctx.JSON(post)
}

func listenEvents() iris.Handler {
	return iris.FromStd(sseServer.ServeHTTP)
}
