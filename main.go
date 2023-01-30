package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
)

func main() {
	ConfigRuntime()
	StartWorkers()
	StartIris()
}

// ConfigRuntime sets the number of operating system threads.
func ConfigRuntime() {
	nuCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(nuCPU)
	fmt.Printf("Running with %d CPUs\n", nuCPU)
}

// StartWorkers start starsWorker by goroutine.
func StartWorkers() {
	go statsWorker()
}

// StartIris starts Iris web server with setting router.
func StartIris() {
	app := iris.New()
	app.Use(recover.New(), rateLimit)
	app.RegisterView(iris.HTML("./resources", ".html"))

	app.HandleDir("/static", "resources/static", iris.DirOptions{
		Compress: true,
	})
	app.Get("/", index)
	app.Get("/room/{roomid}", roomGET)
	app.Post("/room-post/{roomid}", roomPOST)
	app.Get("/events", listenEvents())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app.Listen(":" + port)
}
