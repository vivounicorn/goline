package main

import (
	"fmt"
	"goline/server"
	"net/http"
	"os"
	"time"
)

var Usage = func() {
	fmt.Println("USAGE: goline [config errorfile path] ...")
}

func main() {
	args := os.Args
	if args == nil || len(args) < 2 {
		Usage()
		return
	}

	plugin := &server.Lands{}
	//"..\\conf\\settings.conf"
	fmt.Println(args[1])
	err := plugin.Initialize(args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	server := http.Server{
		Addr:        ":9090",
		Handler:     plugin,
		ReadTimeout: 6 * time.Second,
	}

	server.ListenAndServe()
}
