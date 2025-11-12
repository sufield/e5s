package main

import (
	"fmt"
	"io"
	"log"

	"github.com/sufield/e5s"
)

func main() {
	cfg := "/work/e5s-client.yaml"
	client, shutdown, err := e5s.Client(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()
	resp, err := client.Get("https://e5s-server:8443/")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("%s", string(b))
}
